package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ruhuang/ink/server/internal/ai"
)

// ChatMessage mirrors the AI service message shape so the enhancer can be
// passed straight into the reply pipeline.
type ChatMessage = ai.ChatMessage

const operationTimeout = 2 * time.Second

// Enhancer wraps memory operations for AI message enhancement. It resolves the
// client on every call because an administrator can replace it at runtime.
type Enhancer struct {
	client func() Client
}

// sessionID isolates caller-supplied conversation identifiers by authenticated
// user. Never fall back to legacy unscoped sessions, which may belong to others.
func sessionID(userID, conversationID string) string {
	encoded, _ := json.Marshal([2]string{userID, conversationID})
	return fmt.Sprintf("ink-%x", sha256.Sum256(encoded))
}

// NewEnhancer creates a new memory enhancer.
func NewEnhancer(client func() Client) *Enhancer {
	return &Enhancer{client: client}
}

func (e *Enhancer) resolve() Client {
	if e.client == nil {
		return nil
	}
	return e.client()
}

// EnhanceMessages adds memory context to conversation messages. Failures
// degrade silently: a memory outage must never break a reply.
func (e *Enhancer) EnhanceMessages(ctx context.Context, userID, conversationID string, messages []ChatMessage) ([]ChatMessage, error) {
	if userID == "" || conversationID == "" {
		return messages, nil
	}
	conversationID = sessionID(userID, conversationID)
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	client := e.resolve()
	if client == nil {
		return messages, nil
	}

	if err := client.IsAvailable(ctx); err != nil {
		return messages, nil // silently degrade
	}

	// Get conversation facts if available.
	summary, _ := client.GetSessionSummary(ctx, conversationID)

	// Search for relevant user memories based on last user message
	var relevantMemories []Memory
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == "user" && userID != "" {
			relevantMemories, _ = client.SearchMemories(ctx, userID, lastMsg.Content, 3)
		}
	}

	// Build memory context
	if summary == nil && len(relevantMemories) == 0 {
		return messages, nil // no memory to add
	}

	contextParts := []string{}
	if summary != nil && summary.Summary != "" {
		contextParts = append(contextParts, fmt.Sprintf("**Conversation facts**: %s", summary.Summary))
	}

	if len(relevantMemories) > 0 {
		contextParts = append(contextParts, "\n**Relevant memories**:")
		for _, mem := range relevantMemories {
			contextParts = append(contextParts, fmt.Sprintf("- %s", mem.Content))
		}
	}

	if len(contextParts) == 0 {
		return messages, nil
	}

	// Prepend system message with memory context
	memoryContext := strings.Join(contextParts, "\n")
	systemMsg := ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("## User Memory Context\n\n%s\n\nUse these memories to provide a personalized response.", memoryContext),
	}

	enhanced := make([]ChatMessage, 0, len(messages)+1)
	enhanced = append(enhanced, systemMsg)
	enhanced = append(enhanced, messages...)
	return enhanced, nil
}

// StoreConversation saves conversation messages to memory.
func (e *Enhancer) StoreConversation(ctx context.Context, userID, conversationID string, messages []ChatMessage, assistantReply string) error {
	if userID == "" || conversationID == "" {
		return nil
	}
	conversationID = sessionID(userID, conversationID)
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	client := e.resolve()
	if client == nil {
		return nil
	}

	if err := client.IsAvailable(ctx); err != nil {
		return nil // silently degrade
	}

	// Zep only links a session to a user at creation time; without this the
	// memories are stored but never retrievable for that user.
	if userID != "" && conversationID != "" {
		if err := client.EnsureSession(ctx, conversationID, userID); err != nil {
			return err
		}
	}

	// Store the last user message
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == "user" {
			_ = client.AddMessage(ctx, conversationID, Message{
				Role:    "user",
				Content: lastMsg.Content,
			})
		}
	}

	// Store assistant reply
	if assistantReply != "" {
		_ = client.AddMessage(ctx, conversationID, Message{
			Role:    "assistant",
			Content: assistantReply,
		})
	}

	return nil
}

var _ ai.MemoryEnhancer = (*Enhancer)(nil)
