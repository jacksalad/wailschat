package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"wailschat/internal/model"
)

// SelectionResponder abstracts the App's ability to manage selection channels
// and emit events, breaking the circular dependency between tools and main.
type SelectionResponder interface {
	// RegisterSelectionChannel stores the response channel for a pending selection.
	RegisterSelectionChannel(requestID string, ch chan model.SelectionResponse)
	// DeleteSelectionChannel removes and returns the channel for the given request.
	DeleteSelectionChannel(requestID string) (chan model.SelectionResponse, bool)
	// EmitSelectionRequest emits a Wails event to the frontend to show the selection UI.
	EmitSelectionRequest(requestID, prompt, selectionType string, options []map[string]string, defaultValue any, sessionID int64)
}

// ProvideSelection is a built-in tool that presents interactive choices to the user.
type ProvideSelection struct {
	responder SelectionResponder
}

// NewProvideSelection creates a new ProvideSelection tool.
func NewProvideSelection(responder SelectionResponder) *ProvideSelection {
	return &ProvideSelection{responder: responder}
}

func (t *ProvideSelection) Name() string {
	return "provide_selection"
}

func (t *ProvideSelection) Description() string {
	return "Present interactive choices to the user for selection. Use 'radio' for single choice (user picks one option) or 'checkbox' for multiple choice (user can pick several options). The tool will wait for the user to make a selection and confirm."
}

func (t *ProvideSelection) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "The question or prompt text to display to the user above the options"
			},
			"type": {
				"type": "string",
				"enum": ["radio", "checkbox"],
				"description": "Selection type: 'radio' for single selection, 'checkbox' for multiple selection"
			},
			"options": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"label": {"type": "string", "description": "Display text for this option"},
						"value": {"type": "string", "description": "Value returned when this option is selected"}
					},
					"required": ["label", "value"]
				},
				"description": "Available options for the user to select from"
			},
			"default_value": {
				"description": "Pre-selected value(s). A string for radio type, an array of strings for checkbox type",
				"oneOf": [
					{"type": "string"},
					{"type": "array", "items": {"type": "string"}}
				]
			},
			"session_id": {
				"type": "integer",
				"description": "The current session ID (used internally)"
			}
		},
		"required": ["prompt", "type", "options"]
	}`)
}

func (t *ProvideSelection) Execute(args map[string]any) (string, error) {
	// Extract and validate prompt
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	// Extract and validate type
	selectionType, _ := args["type"].(string)
	if selectionType != "radio" && selectionType != "checkbox" {
		return "", fmt.Errorf("type must be 'radio' or 'checkbox'")
	}

	// Extract and validate options
	optionsRaw, ok := args["options"]
	if !ok {
		return "", fmt.Errorf("options is required")
	}
	optionsSlice, ok := optionsRaw.([]interface{})
	if !ok || len(optionsSlice) == 0 {
		return "", fmt.Errorf("options must be a non-empty array")
	}

	var options []map[string]string
	for i, opt := range optionsSlice {
		optMap, ok := opt.(map[string]any)
		if !ok {
			return "", fmt.Errorf("option %d must be an object", i)
		}
		label, _ := optMap["label"].(string)
		value, _ := optMap["value"].(string)
		if label == "" || value == "" {
			return "", fmt.Errorf("option %d must have both label and value", i)
		}
		options = append(options, map[string]string{"label": label, "value": value})
	}

	// Extract optional default_value
	var defaultValue any
	if dv, ok := args["default_value"]; ok {
		defaultValue = dv
	}

	// Extract optional session_id
	var sessionID int64
	if sid, ok := args["session_id"]; ok {
		switch v := sid.(type) {
		case float64:
			sessionID = int64(v)
		case int64:
			sessionID = v
		}
	}

	// Generate unique request ID
	requestID := uuid.New().String()

	// Create response channel (buffered so writing doesn't block)
	ch := make(chan model.SelectionResponse, 1)

	// Register the channel
	t.responder.RegisterSelectionChannel(requestID, ch)

	// Emit selection request event to frontend
	t.responder.EmitSelectionRequest(requestID, prompt, selectionType, options, defaultValue, sessionID)

	// Block waiting for user response with 10-minute timeout
	select {
	case resp := <-ch:
		if resp.Cancelled {
			result, _ := json.Marshal(map[string]any{
				"selected":  []string{},
				"cancelled": true,
			})
			return string(result), nil
		}
		if selectionType == "radio" {
			// Return single value
			selected := ""
			if len(resp.Selected) > 0 {
				selected = resp.Selected[0]
			}
			result, _ := json.Marshal(map[string]any{
				"selected": selected,
			})
			return string(result), nil
		}
		// Return array for checkbox
		result, _ := json.Marshal(map[string]any{
			"selected": resp.Selected,
		})
		return string(result), nil

	case <-time.After(10 * time.Minute):
		t.responder.DeleteSelectionChannel(requestID)
		return "", fmt.Errorf("selection request timed out after 10 minutes (request_id: %s)", requestID)
	}
}

// extractSessionIDFromToolArgs is a helper to find the session_id in tool arguments.
func extractSessionIDFromToolArgs(args map[string]any) int64 {
	if sid, ok := args["session_id"]; ok {
		switch v := sid.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		}
	}
	return 0
}

// isSelectionCancelled checks if the response indicates cancellation.
func isSelectionCancelled(resp model.SelectionResponse) bool {
	return resp.Cancelled
}

// cleanSelectionPrompt truncates a prompt for safe logging.
func cleanSelectionPrompt(s string) string {
	if len(s) <= 100 {
		return s
	}
	return s[:100] + "..."
}

// joinOptionLabels joins option labels for logging.
func joinOptionLabels(options []map[string]string) string {
	labels := make([]string, len(options))
	for i, opt := range options {
		labels[i] = opt["label"]
	}
	if len(labels) > 5 {
		return strings.Join(labels[:5], ", ") + fmt.Sprintf(" (+%d more)", len(labels)-5)
	}
	return strings.Join(labels, ", ")
}
