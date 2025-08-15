package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
)

// DemoServer implements a test server that transforms Anthropic API requests to OpenAI
// and transforms OpenAI responses back to Anthropic format
type DemoServer struct {
	router      *mux.Router
	openAIHost  string
	openAIKey   string
	config      *aicontext.AnthropicConfig
	modelMapper aicontext.ModelManager
	server      *http.Server
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewDemoServer(openAIHost, openAIKey string) *DemoServer {
	config := aicontext.NewAnthropicConfig()
	modelMapper := aicontext.NewDefaultModelManager()

	// Use environment API key if provided one is empty
	if openAIKey == "" {
		openAIKey = getEnv("OPENAI_API_KEY", "")
	}

	router := mux.NewRouter()

	server := &DemoServer{
		router:      router,
		openAIHost:  openAIHost,
		openAIKey:   openAIKey,
		config:      config,
		modelMapper: modelMapper,
	}

	// Register routes
	router.HandleFunc("/v1/messages", server.handleMessages).Methods("POST")

	return server
}

func (s *DemoServer) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.server.ListenAndServe()
}

func (s *DemoServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *DemoServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Check if API key is configured
	if s.openAIKey == "" {
		http.Error(w, "OpenAI API key not configured. Set OPENAI_API_KEY environment variable.", http.StatusUnauthorized)
		return
	}

	// Read Claude request body
	claudeReqBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading request body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// Parse Claude request
	var claudeReq aicontext.ClaudeMessagesRequest
	if err := json.Unmarshal(claudeReqBody, &claudeReq); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing Claude request: %v", err), http.StatusBadRequest)
		return
	}

	// Print Claude request with indentation
	claudeReqPretty, _ := json.MarshalIndent(claudeReq, "", "  ")
	fmt.Printf("Claude Request:\n%s\n\n", claudeReqPretty)

	// Convert to OpenAI format
	openAIReq, err := aicontext.ConvertClaudeToOpenAI(&claudeReq, s.modelMapper, s.config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error converting to OpenAI format: %v", err), http.StatusInternalServerError)
		return
	}

	// Print OpenAI request with indentation
	openAIReqPretty, _ := json.MarshalIndent(openAIReq, "", "  ")
	fmt.Printf("OpenAI Request:\n%s\n\n", openAIReqPretty)

	// Send request to OpenAI API
	openAIReqBytes, _ := json.Marshal(openAIReq)
	openAIURL := fmt.Sprintf("%s/v1/chat/completions", s.openAIHost)
	openAIReqObj, _ := http.NewRequest("POST", openAIURL, bytes.NewReader(openAIReqBytes))
	openAIReqObj.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.openAIKey))
	openAIReqObj.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 60 * time.Second}
	openAIResp, err := httpClient.Do(openAIReqObj)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error calling OpenAI API: %v", err), http.StatusInternalServerError)
		return
	}

	// Read OpenAI response
	openAIRespBody, err := io.ReadAll(openAIResp.Body)
	openAIResp.Body.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading OpenAI response: %v", err), http.StatusInternalServerError)
		return
	}

	// Check for OpenAI error
	if openAIResp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("OpenAI API error (status %d): %s", openAIResp.StatusCode, openAIRespBody),
			openAIResp.StatusCode)
		return
	}

	// Parse OpenAI response
	var openAIRespMap map[string]interface{}
	if err := json.Unmarshal(openAIRespBody, &openAIRespMap); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing OpenAI response: %v", err), http.StatusInternalServerError)
		return
	}

	// Print OpenAI response with indentation
	openAIRespPretty, _ := json.MarshalIndent(openAIRespMap, "", "  ")
	fmt.Printf("OpenAI Response:\n%s\n\n", openAIRespPretty)

	// Convert back to Claude format
	claudeResp, err := aicontext.ConvertOpenAIToClaudeResponse(openAIRespMap, &claudeReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error converting to Claude format: %v", err), http.StatusInternalServerError)
		return
	}

	// Print Claude response with indentation
	claudeRespPretty, _ := json.MarshalIndent(claudeResp, "", "  ")
	fmt.Printf("Claude Response:\n%s\n\n", claudeRespPretty)

	// Return Claude response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(claudeResp)
}

func main() {
	// Use empty API key - the user will provide their own
	openAIKey := ""

	// Create and start server
	openAIHost := getEnv("OPENAI_API_HOST", "https://api.openai.com")
	server := NewDemoServer(openAIHost, openAIKey)

	go func() {
		fmt.Println("Starting server on http://localhost:8080")
		fmt.Println("You can set OPENAI_API_KEY env variable before running this test")
		err := server.Start(":8080")
		if err != nil && !strings.Contains(err.Error(), "closed") {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	fmt.Println("Server is running on http://localhost:8080")
	fmt.Println("Example curl command:")
	fmt.Println("curl -X POST http://localhost:8080/v1/messages -H \"Content-Type: application/json\" -d '{\"model\":\"claude-3-opus-20240229\",\"max_tokens\":1024,\"messages\":[{\"role\":\"user\",\"content\":\"Hello, how are you?\"}]}'")
	fmt.Println("Press Ctrl+C to stop the server")

	// Wait until manually terminated
	select {}
}
