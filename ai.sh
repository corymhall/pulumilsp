#!/usr/bin/env bash

# Construct the JSON payload using jq
json_payload=$(jq -n --arg query "$(cat <<EOF
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

Code snippet:

new aws.s3.BucketV2("my-bucket", {
  forceDestroy: true,
});

Failed policy check:
Check that S3 Bucket Server-Side Encryption (SSE) is enabled.
S3 Buckets Server-Side Encryption (SSE) should be enabled.

EOF
)" '{
  query: $query,
  directSkillCall: {
    skill: "codeGen",
    params: {
      validateResult: false,
      validationStopPhase: "new",
      selfDebugMaxIterations: 1,
      selfDebugMaxNumberOfErrors: 10,
      selfDebugSupportedLanguages: ["TypeScript"],
      getProjectFiles: false,
      customInstructions: $query
    }
  },
  state: {
    client: {
      cloudContext: {
        orgId: "pulumi",
        url: "https://api.pulumi.com"
      }
    }
  }
}')

# Make the API request
curl -L https://api.pulumi.com/api/ai/chat/preview \
-H "Authorization: token $PULUMI_WORK_TOKEN" \
-H "Content-Type: application/json" \
-d "$json_payload"
