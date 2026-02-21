package agent

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/tools"
)

// RPCService handles RPC requests with panic recovery
type RPCService struct {
	agent *Agent
}

func NewRPCService(a *Agent) *RPCService {
	return &RPCService{agent: a}
}

// recoverWrap wraps a function with panic recovery
func recoverWrap(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[WARN] RPC panic recovered: %v", r)
		}
	}()
	err = fn()
	return
}

func (s *RPCService) Chat(args *rpcproto.ChatArgs, reply *rpcproto.ChatReply) error {
	return recoverWrap(func() error {
		if s.agent == nil {
			return fmt.Errorf("agent not initialized")
		}

		// Use session_key from args, default to "default"
		sessionKey := args.SessionKey
		if sessionKey == "" {
			sessionKey = "default"
		}

		msgs := make([]Message, len(args.Messages))
		for i, m := range args.Messages {
			msgs[i] = Message{
				Role:    m.Role,
				Content: m.Content,
			}
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

		reply.Content = s.agent.ChatWithSession(sessionKey, msgs)
		return nil
	})
}

func (s *RPCService) Stats(_ struct{}, reply *rpcproto.StatsReply) error {
	return recoverWrap(func() error {
		if s.agent == nil || s.agent.Store() == nil {
			return fmt.Errorf("storage not initialized")
		}
		stats, err := s.agent.Store().Stats()
		if err != nil {
			return err
		}
		// Convert map[string]int to map[string]int32
		stats32 := make(map[string]int32)
		for k, v := range stats {
			stats32[k] = int32(v)
		}
		reply.Stats = stats32
		return nil
	})
}

func (s *RPCService) MemorySearch(args *rpcproto.MemorySearchArgs, reply *rpcproto.ToolResultReply) error {
	return recoverWrap(func() error {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return fmt.Errorf("memory store not initialized")
		}

		tool := tools.NewMemoryTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{
			"query":    args.Query,
			"category": args.Category,
			"limit":    args.Limit,
			"minScore": args.MinScore,
		})
		if err != nil {
			return err
		}
		jsonBytes, _ := json.Marshal(result)
		reply.Result = string(jsonBytes)
		return nil
	})
}

func (s *RPCService) MemoryGet(args *rpcproto.MemoryGetArgs, reply *rpcproto.ToolResultReply) error {
	return recoverWrap(func() error {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return fmt.Errorf("memory store not initialized")
		}

		tool := tools.NewMemoryGetTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{"path": args.Path})
		if err != nil {
			return err
		}
		jsonBytes, _ := json.Marshal(result)
		reply.Result = string(jsonBytes)
		return nil
	})
}

func (s *RPCService) MemoryStore(args *rpcproto.MemoryStoreArgs, reply *rpcproto.ToolResultReply) error {
	return recoverWrap(func() error {
		if s.agent == nil || s.agent.MemoryStore() == nil {
			return fmt.Errorf("memory store not initialized")
		}

		tool := tools.NewMemoryStoreTool(s.agent.MemoryStore())
		result, err := tool.Execute(map[string]interface{}{
			"text":       args.Text,
			"category":   args.Category,
			"importance": args.Importance,
		})
		if err != nil {
			return err
		}
		jsonBytes, _ := json.Marshal(result)
		reply.Result = string(jsonBytes)
		return nil
	})
}

// PulseAdd adds a new pulse event
func (s *RPCService) PulseAdd(args *rpcproto.PulseArgs, reply *rpcproto.PulseReply) error {
	return recoverWrap(func() error {
		if s.agent == nil {
			return fmt.Errorf("agent not initialized")
		}

		eventID, err := s.agent.AddPulseEvent(args.Title, args.Content, int(args.Priority), args.Channel)
		if err != nil {
			return err
		}

		reply.EventId = eventID
		reply.Result = "Event added successfully"
		reply.Status = "pending"
		return nil
	})
}

// PulseStatus returns the current pulse system status
func (s *RPCService) PulseStatus(args struct{}, reply *rpcproto.PulseReply) error {
	return recoverWrap(func() error {
		if s.agent == nil {
			return fmt.Errorf("agent not initialized")
		}

		status, err := s.agent.GetPulseStatus()
		if err != nil {
			return err
		}

		data, _ := json.Marshal(status)
		reply.Result = string(data)
		return nil
	})
}
