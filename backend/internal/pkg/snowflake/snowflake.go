package snowflake

import "github.com/bwmarrin/snowflake"

type Generator struct {
	node *snowflake.Node
}

func New(nodeID int64) (*Generator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, err
	}
	return &Generator{node: node}, nil
}

func (g *Generator) Next() int64 {
	return g.node.Generate().Int64()
}
