package rpcproto

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentGRPCClient is the generated gRPC client.
type AgentGRPCClient struct {
	conn   *grpc.ClientConn
	client AgentClient
}

func NewAgentGRPCClient(conn *grpc.ClientConn) *AgentGRPCClient {
	return &AgentGRPCClient{
		conn:   conn,
		client: NewAgentClient(conn),
	}
}

func (c *AgentGRPCClient) Chat(ctx context.Context, args *ChatArgs) (*ChatReply, error) {
	resp, err := c.client.Chat(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) Stats(ctx context.Context) (*StatsReply, error) {
	resp, err := c.client.Stats(ctx, &StatsArgs{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) ChatStream(ctx context.Context, args *ChatArgs) (Agent_ChatStreamClient, error) {
	stream, err := c.client.ChatStream(ctx, args)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (c *AgentGRPCClient) Sessions(ctx context.Context, args *SessionsArgs) (*SessionsReply, error) {
	resp, err := c.client.Sessions(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) MemorySearch(ctx context.Context, args *MemorySearchArgs) (*ToolResultReply, error) {
	resp, err := c.client.MemorySearch(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) MemoryGet(ctx context.Context, args *MemoryGetArgs) (*ToolResultReply, error) {
	resp, err := c.client.MemoryGet(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) MemoryStore(ctx context.Context, args *MemoryStoreArgs) (*ToolResultReply, error) {
	resp, err := c.client.MemoryStore(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) PulseAdd(ctx context.Context, args *PulseArgs) (*PulseReply, error) {
	resp, err := c.client.PulseAdd(ctx, args)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) PulseStatus(ctx context.Context) (*PulseReply, error) {
	resp, err := c.client.PulseStatus(ctx, &PulseArgs{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *AgentGRPCClient) SendAudioChunk(ctx context.Context, args *AudioChunkArgs) (*AudioReply, error) {
	return c.client.SendAudioChunk(ctx, args)
}

func (c *AgentGRPCClient) EndAudioStream(ctx context.Context, args *AudioArgs) (*AudioReply, error) {
	return c.client.EndAudioStream(ctx, args)
}

// DialAgent connects to the agent via gRPC Unix socket.
func DialAgent(addr string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(
		"unix://"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial failed: %w", err)
	}

	// Wait for connection to reach Ready state with backoff
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			conn.Close()
			return nil, fmt.Errorf("grpc connection timeout: still in %s state", conn.GetState())
		}

		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return conn, nil
		case connectivity.TransientFailure, connectivity.Shutdown:
			conn.Close()
			return nil, fmt.Errorf("grpc connection failed: %s", state)
		case connectivity.Idle:
			conn.Connect() // Trigger reconnect
		case connectivity.Connecting:
			// Wait for state change
		}

		// Wait a bit before checking again
		if !conn.WaitForStateChange(ctx, state) {
			if ctx.Err() != nil {
				conn.Close()
				return nil, fmt.Errorf("grpc connection context cancelled: %w", ctx.Err())
			}
		}
	}
}

func DefaultGRPCTimeout() time.Duration {
	return 60 * time.Second
}

// ConvertStats converts map[string]int32 to map[string]int
func ConvertStats(m map[string]int32) map[string]int {
	result := make(map[string]int)
	for k, v := range m {
		result[k] = int(v)
	}
	return result
}

// ToMessagesPtr converts []Message to []*Message
func ToMessagesPtr(messages []Message) []*Message {
	result := make([]*Message, len(messages))
	for i := range messages {
		result[i] = &messages[i]
	}
	return result
}
