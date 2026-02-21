package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/tools"
)

// GRPCService implements the generated gRPC server interface.
type GRPCService struct {
	agent *Agent
	rpcproto.UnimplementedAgentServer
}

func NewGRPCService(a *Agent) *GRPCService {
	return &GRPCService{agent: a}
}

func (s *GRPCService) Chat(ctx context.Context, args *rpcproto.ChatArgs) (*rpcproto.ChatReply, error) {
	return wrapGRPC(func() (*rpcproto.ChatReply, error) {
		if s.agent == nil {
			return nil, fmt.Errorf("agent not initialized")
		}

		msgs := make([]Message, len(args.Messages))
		for i, m := range args.Messages {
			msgs[i] = Message{Role: m.Role, Content: m.Content}
			if len(m.ToolCalls) > 0 {
				msgs[i].ToolCalls = make([]ToolCall, len(m.ToolCalls))
				for j, c := range m.ToolCalls {
					msgs[i].ToolCalls[j] = ToolCall{
						ID:   c.Id,
						Type: c.Type,
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      c.Function.Name,
							Arguments: c.Function.Arguments,
						},
					}
				}
			}
			if len(m.ToolExecutionResults) > 0 {
				msgs[i].ToolExecutionResults = make([]ToolResult, len(m.ToolExecutionResults))
				for j, r := range m.ToolExecutionResults {
					msgs[i].ToolExecutionResults[j] = ToolResult{ID: r.Id, Type: r.Type, Result: r.Result}
				}
			}
		}

		reply := s.agent.Chat(msgs)
		return &rpcproto.ChatReply{Content: reply}, nil
	})
}

func (s *GRPCService) Stats(ctx context.Context, _ *rpcproto.StatsArgs) (*rpcproto.StatsReply, error) {
	return wrapGRPCStats(func() (*rpcproto.StatsReply, error) {
		if s.agent == nil || s.agent.Store() == nil {
			return nil, fmt.Errorf("storage not initialized")
		}
		stats, err := s.agent.Store().Stats()
		if err != nil {
			return nil, err
		}
		// Convert map[string]int to map[string]int32 for protobuf
		stats32 := make(map[string]int32)
		for k, v := range stats {
			stats32[k] = int32(v)
		}
		return &rpcproto.StatsReply{Stats: stats32}, nil
	})
}

func (s *GRPCService) Sessions(ctx context.Context, args *rpcproto.SessionsArgs) (*rpcproto.SessionsReply, error) {
	if s.agent == nil || s.agent.Store() == nil {
		return nil, fmt.Errorf("storage not initialized")
	}
	sessions, err := s.agent.Store().GetAllSessions()
	if err != nil {
		return nil, err
	}
	reply := &rpcproto.SessionsReply{Count: int32(len(sessions))}
	for _, ses := range sessions {
		reply.Sessions = append(reply.Sessions, &rpcproto.SessionInfo{
			SessionKey:      ses.SessionKey,
			TotalTokens:     int32(ses.TotalTokens),
			CompactionCount: int32(ses.CompactionCount),
			UpdatedAt:       ses.UpdatedAt.Format(time.RFC3339),
		})
	}
	return reply, nil
}

func (s *GRPCService) ChatStream(args *rpcproto.ChatArgs, stream rpcproto.Agent_ChatStreamServer) error {
	if s.agent == nil {
		return fmt.Errorf("agent not initialized")
	}

	msgs := make([]Message, len(args.Messages))
	for i, m := range args.Messages {
		msgs[i] = Message{Role: m.Role, Content: m.Content}
		if len(m.ToolCalls) > 0 {
			msgs[i].ToolCalls = make([]ToolCall, len(m.ToolCalls))
			for j, c := range m.ToolCalls {
				msgs[i].ToolCalls[j] = ToolCall{
					ID:   c.Id,
					Type: c.Type,
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      c.Function.Name,
						Arguments: c.Function.Arguments,
					},
				}
			}
		}
		if len(m.ToolExecutionResults) > 0 {
			msgs[i].ToolExecutionResults = make([]ToolResult, len(m.ToolExecutionResults))
			for j, r := range m.ToolExecutionResults {
				msgs[i].ToolExecutionResults[j] = ToolResult{ID: r.Id, Type: r.Type, Result: r.Result}
			}
		}
	}

	// Use streaming callback
	s.agent.ChatStream(msgs, func(chunk string) {
		if err := stream.Send(&rpcproto.ChatStreamReply{Content: chunk, Done: false}); err != nil {
			log.Printf("[GRPC] stream send error: %v", err)
		}
	})

	// Send done signal
	if err := stream.Send(&rpcproto.ChatStreamReply{Content: "", Done: true}); err != nil {
		return err
	}
	return nil
}

func (s *GRPCService) MemorySearch(ctx context.Context, args *rpcproto.MemorySearchArgs) (*rpcproto.ToolResultReply, error) {
	return wrapGRPCMem(func() (*rpcproto.ToolResultReply, error) {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return nil, fmt.Errorf("memory store not initialized")
		}
		tool := tools.NewMemoryTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{
			"query":    args.Query,
			"category": args.Category,
			"limit":    int(args.Limit),
			"minScore": float64(args.MinScore),
		})
		if err != nil {
			return nil, err
		}
		jsonBytes, _ := json.Marshal(result)
		return &rpcproto.ToolResultReply{Result: string(jsonBytes)}, nil
	})
}

func (s *GRPCService) MemoryGet(ctx context.Context, args *rpcproto.MemoryGetArgs) (*rpcproto.ToolResultReply, error) {
	return wrapGRPCMem(func() (*rpcproto.ToolResultReply, error) {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return nil, fmt.Errorf("memory store not initialized")
		}
		tool := tools.NewMemoryGetTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{"path": args.Path})
		if err != nil {
			return nil, err
		}
		jsonBytes, _ := json.Marshal(result)
		return &rpcproto.ToolResultReply{Result: string(jsonBytes)}, nil
	})
}

func (s *GRPCService) MemoryStore(ctx context.Context, args *rpcproto.MemoryStoreArgs) (*rpcproto.ToolResultReply, error) {
	return wrapGRPCMem(func() (*rpcproto.ToolResultReply, error) {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return nil, fmt.Errorf("memory store not initialized")
		}
		tool := tools.NewMemoryStoreTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{
			"text":       args.Text,
			"category":   args.Category,
			"importance": float64(args.Importance),
		})
		if err != nil {
			return nil, err
		}
		jsonBytes, _ := json.Marshal(result)
		return &rpcproto.ToolResultReply{Result: string(jsonBytes)}, nil
	})
}

func (s *GRPCService) PulseAdd(ctx context.Context, args *rpcproto.PulseArgs) (*rpcproto.PulseReply, error) {
	return wrapGRPCPulse(func() (*rpcproto.PulseReply, error) {
		if s.agent == nil {
			return nil, fmt.Errorf("agent not initialized")
		}
		eventID, err := s.agent.AddPulseEvent(args.Title, args.Content, int(args.Priority), args.Channel)
		if err != nil {
			return nil, err
		}
		return &rpcproto.PulseReply{EventId: eventID, Result: "Event added successfully", Status: "pending"}, nil
	})
}

func (s *GRPCService) PulseStatus(ctx context.Context, _ *rpcproto.PulseArgs) (*rpcproto.PulseReply, error) {
	return wrapGRPCPulse(func() (*rpcproto.PulseReply, error) {
		if s.agent == nil {
			return nil, fmt.Errorf("agent not initialized")
		}
		status, err := s.agent.GetPulseStatus()
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(status)
		return &rpcproto.PulseReply{Result: string(data)}, nil
	})
}

func (s *GRPCService) SendAudioChunk(ctx context.Context, args *rpcproto.AudioChunkArgs) (*rpcproto.AudioReply, error) {
	if s.agent == nil {
		return &rpcproto.AudioReply{Error: "agent not initialized"}, nil
	}
	if args.SessionKey == "" {
		return &rpcproto.AudioReply{Error: "session_key required"}, nil
	}
	if len(args.AudioData) == 0 {
		return &rpcproto.AudioReply{}, nil
	}
	err := s.agent.SendAudioChunk(args.SessionKey, args.AudioData)
	if err != nil {
		return &rpcproto.AudioReply{Error: err.Error()}, nil
	}
	return &rpcproto.AudioReply{}, nil
}

func (s *GRPCService) EndAudioStream(ctx context.Context, args *rpcproto.AudioArgs) (*rpcproto.AudioReply, error) {
	if s.agent == nil {
		return &rpcproto.AudioReply{Error: "agent not initialized"}, nil
	}
	if args.SessionKey == "" {
		return &rpcproto.AudioReply{Error: "session_key required"}, nil
	}
	err := s.agent.EndAudioStream(args.SessionKey)
	if err != nil {
		return &rpcproto.AudioReply{Error: err.Error()}, nil
	}
	return &rpcproto.AudioReply{}, nil
}

func wrapGRPC(fn func() (*rpcproto.ChatReply, error)) (resp *rpcproto.ChatReply, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[WARN] gRPC panic recovered: %v", r)
		}
	}()
	return fn()
}

func wrapGRPCStats(fn func() (*rpcproto.StatsReply, error)) (resp *rpcproto.StatsReply, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[WARN] gRPC panic recovered: %v", r)
		}
	}()
	return fn()
}

func wrapGRPCMem(fn func() (*rpcproto.ToolResultReply, error)) (resp *rpcproto.ToolResultReply, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[WARN] gRPC panic recovered: %v", r)
		}
	}()
	return fn()
}

func wrapGRPCPulse(fn func() (*rpcproto.PulseReply, error)) (resp *rpcproto.PulseReply, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[WARN] gRPC panic recovered: %v", r)
		}
	}()
	return fn()
}
