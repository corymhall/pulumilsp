package parser

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func TestParser(t *testing.T) {
	text := "import * as aws from '@pulumi/aws';\n\nnew aws.s3.Bucket('my-bucket', {\n  serverSideEncryptionConfiguration: {\n    rule: {\n      applyServerSideEncryptionByDefault: {\n        sseAlgorithm: 'AES256'\n      }\n    }\n  }\n});\n\nnew aws.s3.Bucket('my-bucket2');\n"
	lang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	napper, err := NewResourceNapper(lang)
	require.NoError(t, err)
	captures, err := napper.GetCapturesFromFile([]byte(text))
	require.NoError(t, err)
	require.Len(t, captures, 2)
	autogold.Expect([]CaptureInfo{
		{
			ResourceName:     "my-bucket",
			ResourceTypeName: "Bucket",
			Text: `new aws.s3.Bucket('my-bucket', {
  serverSideEncryptionConfiguration: {
    rule: {
      applyServerSideEncryptionByDefault: {
        sseAlgorithm: 'AES256'
      }
    }
  }
});`,
			StartPoint: tree_sitter.Point{Row: 2},
			EndPoint: tree_sitter.Point{
				Row:    10,
				Column: 3,
			},
		},
		{
			ResourceName:     "my-bucket2",
			ResourceTypeName: "Bucket",
			Text:             "new aws.s3.Bucket('my-bucket2');",
			StartPoint:       tree_sitter.Point{Row: 12},
			EndPoint: tree_sitter.Point{
				Row:    12,
				Column: 32,
			},
		},
	}).Equal(t, captures)
}
