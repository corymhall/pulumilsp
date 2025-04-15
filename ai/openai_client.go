package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type OpenAIResponse struct {
	Status string         `json:"status"`
	Error  string         `json:"error"`
	Output []OpenAIOutput `json:"output"`
}
type OpenAIOutput struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []OpenAIContent `json:"content"`
}

type OpenAIContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func FixWithOpenAI(ctx context.Context, content, problem string) (string, error) {
	client := http.DefaultClient

	request := OpenAIRequest{
		Model: "gpt-4o",
		Input: fmt.Sprintf(`
I have a code snippet below that has failed a policy check. You need to fix the code so that it
will no longer fail the policy check. The code snippet is only a snippet of code that belongs
to a much larger file with other resources so do not add any code other than what is strictly necessary.
You should only return the code snippet that has been fixed.
Do not include any other text in your response. Do not include any comments, imports, exports, etc.

Here are specific instructions for the code generation:

- You should never change the type of resource. For example do not change "aws.s3.BucketV2" to "aws.s3.Bucket".
- You should always use side car resources if they are available. For example do not set the "serverSideEncryptionConfigurations"
property on a BucketV2 resource, instead use the "aws.s3.BucketServerSideEncryptionConfigurationV2" resource.
- You should not change the "id" of a resource, for example do not change "new aws.s3.BucketV2('my-bucket')" to "new aws.s3.BucketV2('other-bucket')"
- You should not add a "name" property to a resource. For buckets that would be the "bucket" property.
- Do not include import statements like "import * as aws from '@pulumi/aws'". These already exist in the file the snippet belongs to
- You should use resource references instead of hardcoded strings. For example, You should do this:

// good
const bucket = new aws.s3.BucketV2('my-bucket');
new aws.s3.BucketServerSideEncryptionConfigurationV2('my-bucket-sse', {
    bucket: bucket.id, // bucket reference
    rules: [{
        applyServerSideEncryptionByDefault: {
            sseAlgorithm: 'AES256',
        },
    }],
});

instead of this:

// bad
const bucket = new aws.s3.BucketV2('my-bucket');
new aws.s3.BucketServerSideEncryptionConfigurationV2('my-bucket-sse', {
    bucket: "my-bucket", // hardcoded string bad!
    rules: [{
        applyServerSideEncryptionByDefault: {
            sseAlgorithm: 'AES256',
        },
    }],
});


Code snippet:

%s

Failed policy check:
%s

		`, content, problem),
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	apiToken := os.Getenv("OPENAI_API_KEY")
	if apiToken == "" {
		return "", fmt.Errorf("missing OPENAI_API_KEY environment variable")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var response OpenAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshalling response: %w", err)
	}
	if len(response.Output) == 1 {
		if len(response.Output[0].Content) == 1 {
			return processResult(response.Output[0].Content[0].Text), nil
		}
	}
	return "", fmt.Errorf("unexpected response format: %v", response)
}

// the openai response includes ```javascript``` wrapping
// the code snippet. This function removes that wrapping
func processResult(result string) string {
	lines := strings.Split(result, "\n")
	finalLines := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			continue
		}
		finalLines = append(finalLines, line)
	}

	return strings.Join(finalLines, "\n")
}
