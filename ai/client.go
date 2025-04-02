package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type apiAccessToken string

const (
	PulumiCloudURL         = "https://" + defaultAPIDomainPrefix + "pulumi.com"
	defaultAPIDomainPrefix = "api."
)

type Client struct {
	apiURL     string
	apiToken   apiAccessToken
	httpClient *http.Client
	logger     *log.Logger
}

func NewClient(logger *log.Logger) *Client {
	account, err := workspace.GetAccount(PulumiCloudURL)
	contract.AssertNoErrorf(err, "failed to get account for %s", PulumiCloudURL)
	httpClient := http.DefaultClient
	return &Client{
		logger:     logger,
		apiURL:     PulumiCloudURL,
		apiToken:   apiAccessToken(account.AccessToken),
		httpClient: httpClient,
	}
}

// do proxies the http client's Do method. This is a low-level construct and should be used sparingly
func (pc *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return pc.httpClient.Do(req.WithContext(ctx))
}

// SummarizeErrorWithCopilot summarizes Pulumi Update output using the Copilot API
func (pc *Client) FixWithCopilot2(
	ctx context.Context,
	orgID string,
	content string,
	problem string,
) (string, error) {
	request, err := createFixRequest2(content, orgID, problem, maxCopilotContentLength)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}
	pc.logger.Printf("request: %s", string(jsonData))

	// Requests that take longer that 10 seconds will result in this message being printed to the user:
	// "Error summarizing update output: making request: Post "https://api.pulumi.com/api/ai/chat/preview":
	// context deadline exceeded" Copilot backend will see this in telemetry as well
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := pc.apiURL + "/api/ai/chat/preview"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Source", "Pulumi CLI")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", pc.apiToken))

	resp, err := pc.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// Copilot API returns 204 No Content when it decided that it should not summarize the input.
		// This can happen when the input is too short or Copilot thinks it cannot make it any better.
		// In this case, we will not show the summary to the user. This is better than showing a useless summary.
		return "", nil
	}

	// Read the body first so we can use it for error reporting if needed
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	var copilotResp apitype.CopilotSummarizeUpdateResponse
	if err := json.Unmarshal(body, &copilotResp); err != nil {
		return "", fmt.Errorf("got non-JSON response from Copilot: %s", body)
	}

	if copilotResp.Error != "" {
		return "", fmt.Errorf("copilot API error: %s\n%s", copilotResp.Error, copilotResp.Details)
	}

	return extractSummaryFromResponse(pc.logger, copilotResp)
}

// SummarizeErrorWithCopilot summarizes Pulumi Update output using the Copilot API
func (pc *Client) FixWithCopilot(
	ctx context.Context,
	orgID string,
	content string,
	problem string,
) (string, error) {
	request, err := createFixRequest(content, orgID, problem, maxCopilotContentLength)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}
	pc.logger.Printf("request: %s", string(jsonData))

	// Requests that take longer that 10 seconds will result in this message being printed to the user:
	// "Error summarizing update output: making request: Post "https://api.pulumi.com/api/ai/chat/preview":
	// context deadline exceeded" Copilot backend will see this in telemetry as well
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	url := pc.apiURL + "/api/ai/chat/preview"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Source", "Pulumi LSP")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", pc.apiToken))

	resp, err := pc.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// Copilot API returns 204 No Content when it decided that it should not summarize the input.
		// This can happen when the input is too short or Copilot thinks it cannot make it any better.
		// In this case, we will not show the summary to the user. This is better than showing a useless summary.
		return "", nil
	}

	// Read the body first so we can use it for error reporting if needed
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	var copilotResp apitype.CopilotSummarizeUpdateResponse
	if err := json.Unmarshal(body, &copilotResp); err != nil {
		return "", fmt.Errorf("got non-JSON response from Copilot: %s", body)
	}

	if copilotResp.Error != "" {
		return "", fmt.Errorf("copilot API error: %s\n%s", copilotResp.Error, copilotResp.Details)
	}

	return extractSummaryFromResponse(pc.logger, copilotResp)
}

// createFixRequest creates a new CopilotSummarizeUpdateRequest with the given content and org ID
func createFixRequest2(
	content string,
	orgID string,
	problem string,
	maxUpdateOutputLen int,
) (*CopilotRequest, error) {
	// Convert lines to a single string
	if len(content) > maxUpdateOutputLen {
		return nil, errors.New("Content is too long")
	}

	return &CopilotRequest{
		State: CopilotState{
			Client: CopilotClientState{
				CloudContext: CopilotCloudContext{
					OrgID: orgID,
					URL:   "https://app.pulumi.com",
				},
			},
		},
		Query: fmt.Sprintf(`
I have a code snippet below that has failed a policy check. You need to fix the code so that it
will no longer fail the policy check. You should only return the code snippet that has been fixed.
Do not include any other text in your response. Do not include any comments, imports, exports, etc.

You should never change the type of resource. For example do not change "aws.s3.BucketV2" to "aws.s3.Bucket".
You should always use side car resources if they are available. For example do not set the "serverSideEncryptionConfigurations"
property on a BucketV2 resource, instead use the "aws.s3.BucketServerSideEncryptionConfigurationV2" resource.

Code snippet:

%s

Failed policy check:
%s
`, content, problem),
	}, nil
}

// createFixRequest creates a new CopilotSummarizeUpdateRequest with the given content and org ID
func createFixRequest(
	content string,
	orgID string,
	problem string,
	maxUpdateOutputLen int,
) (*CopilotCodeFixRequest, error) {
	// Convert lines to a single string
	if len(content) > maxUpdateOutputLen {
		return nil, errors.New("Content is too long")
	}

	return &CopilotCodeFixRequest{
		CopilotRequest: CopilotRequest{
			State: CopilotState{
				Client: CopilotClientState{
					CloudContext: CopilotCloudContext{
						OrgID: orgID,
						URL:   "https://app.pulumi.com",
					},
				},
			},
			Query: content,
		},
		DirectSkillCall: CopilotDirectSkillCall{
			Skill: "codeGen",
			Params: CopilotSkillParams{
				ValidateResult:              true,
				ValidationStopPhase:         "typecheck",
				SelfDebugMaxIterations:      2,
				SelfDebugMaxNumberOfErrors:  10,
				SelfDebugSupportedLanguages: []string{"TypeScript"},
				GetProjectFiles:             false,
				CustomInstructions: fmt.Sprintf(`
I have a code snippet below that has failed a policy check. You need to fix the code so that it
will no longer fail the policy check. You should only return the code snippet that has been fixed.
Do not include any other text in your response. Do not include any comments, imports, exports, etc.

You should never change the type of resource. For example do not change "aws.s3.BucketV2" to "aws.s3.Bucket".
You should always use side car resources if they are available. For example do not set the "serverSideEncryptionConfigurations"
property on a BucketV2 resource, instead use the "aws.s3.BucketServerSideEncryptionConfigurationV2" resource.

Code snippet:

%s

Failed policy check:
%s
				`, content, problem),
			},
		},
	}, nil
}

type CopilotMessage struct {
	Code     string             `json:"code"`
	Plan     CopilotMessagePlan `json:"plan"`
	Language string             `json:"language"`
}
type CopilotMessagePlan struct {
	Instructions string `json:"instructions"`
}

// extractSummaryFromResponse parses the Copilot API response and extracts the summary content
func extractSummaryFromResponse(logger *log.Logger, copilotResp apitype.CopilotSummarizeUpdateResponse) (string, error) {
	var finalMessage string
	for _, msg := range copilotResp.ThreadMessages {
		logger.Printf("copilot message: %s - %s - %s", msg.Content, msg.Kind, msg.Role)
		if msg.Role != "assistant" {
			continue
		}

		// Handle the new format where content is a string directly
		if msg.Kind == "program" {
			// Unmarshal the RawMessage into a string
			var message CopilotMessage
			if err := json.Unmarshal(msg.Content, &message); err != nil {
				// If it's not a simple string, it might be a raw JSON object Return it as a string representation
				continue
			}
			if message.Code != "" {
				finalMessage = message.Code
			}
		}
	}
	if finalMessage != "" {
		return processCopilotResult(finalMessage), nil
	}
	return "", errors.New("no assistant message found in response")
}

// Maximum number of characters to send to Copilot. We do this to avoid including a proper token counting library for
// now. Tokens are 3-4 characters as a rough estimate. So this is 1000 tokens.
const maxCopilotContentLength = 4000

// copilot will not obey my instructions and will always
// return import statements at the top and exports at the bottom.
// This is a workaround to remove them.
func processCopilotResult(result string) string {
	lines := strings.Split(result, "\n")
	var processedLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "import") ||
			strings.HasPrefix(line, "export") ||
			strings.HasPrefix(line, "//") ||
			line == "" {
			continue
		}
		processedLines = append(processedLines, line)
	}
	return strings.Join(processedLines, "\n")
}
