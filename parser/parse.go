package parser

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const RESOURCE_QUERY = `
(new_expression
  constructor: [
    (identifier) @resource_name
    (member_expression
      property: (property_identifier) @resource_name
    )
  ]
  arguments: (
    arguments
      (string (string_fragment) @resource_id)?
      (object)? @object_arg
  )
) @resource_code
`

type ResourceNapper struct {
	parser *tree_sitter.Parser
	lang   *tree_sitter.Language
}

func (r *ResourceNapper) Close() {
	if r.parser != nil {
		r.parser.Close()
	}
}

func NewResourceNapper(language *tree_sitter.Language) (*ResourceNapper, error) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}
	parser.StopPrintingDotGraphs()
	return &ResourceNapper{
		parser: parser,
		lang:   language,
	}, nil
}

type CaptureInfo struct {
	ResourceName     string
	ResourceTypeName string
	Text             string
	StartPoint       tree_sitter.Point
	EndPoint         tree_sitter.Point
}

func (r *ResourceNapper) GetCapturesFromFile(fileText []byte) ([]CaptureInfo, error) {
	tree := r.parser.Parse(fileText, nil)
	defer tree.Close()
	query, queryErr := tree_sitter.NewQuery(r.lang, RESOURCE_QUERY)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to create query: %w", queryErr)
	}
	defer query.Close()

	captures := []CaptureInfo{}

	cursor := tree_sitter.NewQueryCursor()
	matches := cursor.Matches(query, tree.RootNode(), nil)
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		info := CaptureInfo{}
		idx, ok := query.CaptureIndexForName("resource_code")
		if !ok {
			continue
		}
		nodes := match.NodesForCaptureIndex(idx)
		if len(nodes) != 1 {
			// it should be a single node
			continue
		}
		node := nodes[0]
		info.StartPoint = node.Range().StartPoint
		info.EndPoint = node.Range().EndPoint
		info.Text = node.Utf8Text(fileText)

		nameIdx, ok := query.CaptureIndexForName("resource_name")
		if !ok {
			// we should have a resource_name
			continue
		}
		nameNodes := match.NodesForCaptureIndex(nameIdx)
		if len(nameNodes) != 1 {
			// it should be a single node
			continue
		}
		nameNode := nameNodes[0]
		info.ResourceTypeName = nameNode.Utf8Text(fileText)

		idIdx, ok := query.CaptureIndexForName("resource_id")
		if !ok {
			// we should have a resource_id
			continue
		}
		idNodes := match.NodesForCaptureIndex(idIdx)
		if len(idNodes) != 1 {
			// it should be a single node
			continue
		}
		idNode := idNodes[0]
		info.ResourceName = idNode.Utf8Text(fileText)

		captures = append(captures, info)
	}
	if len(captures) == 0 {
		return nil, fmt.Errorf("no match found")
	}
	return captures, nil
}
