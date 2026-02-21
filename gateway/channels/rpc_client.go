package channels

import (
	"context"

	"github.com/gliderlab/cogate/rpcproto"
	"google.golang.org/grpc"
)

// RPCClient implements AgentRPCInterface using RPC calls to the agent
type RPCClient struct {
	client *grpc.ClientConn
}

// NewRPCClient creates a new RPC client for agent communication
func NewRPCClient(client *grpc.ClientConn) *RPCClient {
	return &RPCClient{client: client}
}

// Chat sends a chat request to the agent via RPC
func (r *RPCClient) Chat(messages []rpcproto.Message) (string, error) {
	grpcClient := rpcproto.NewAgentGRPCClient(r.client)
	ctx, cancel := context.WithTimeout(context.Background(), rpcproto.DefaultGRPCTimeout())
	defer cancel()

	// Convert []Message to []*Message
	msgPtrs := make([]*rpcproto.Message, len(messages))
	for i := range messages {
		msgPtrs[i] = &messages[i]
	}

	args := rpcproto.ChatArgs{Messages: msgPtrs}
	reply, err := grpcClient.Chat(ctx, &args)
	if err != nil {
		return "", err
	}

	return reply.Content, nil
}

// GetStats gets statistics from the agent via RPC
func (r *RPCClient) GetStats() (map[string]int, error) {
	grpcClient := rpcproto.NewAgentGRPCClient(r.client)
	ctx, cancel := context.WithTimeout(context.Background(), rpcproto.DefaultGRPCTimeout())
	defer cancel()

	reply, err := grpcClient.Stats(ctx)
	if err != nil {
		return nil, err
	}

	// Convert map[string]int32 to map[string]int
	stats := make(map[string]int)
	for k, v := range reply.Stats {
		stats[k] = int(v)
	}
	return stats, nil
}
