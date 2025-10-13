package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/bedrock"
)

func main() {
	ctx := context.Background()

	// Initialize Bedrock LLM with Nova Lite model
	llm, err := bedrock.New(
		bedrock.WithModel("apac.amazon.nova-micro-v1:0"), // Use Nova Lite for fast multimodal
	)
	if err != nil {
		log.Fatal(err)
	}

	// Simple text prompt
	userPrompt := "Describe a futuristic cityscape in vivid detail."

	// Check if the model is Amazon Nova
	modelName := "apac.amazon.nova-micro-v1:0" // Hardcoded model name; adjust if accessed dynamically from llm
	if strings.Contains(modelName, "amazon.nova") {
		// For Nova models, use GenerateContent with system and human messages
		prompt := []llms.MessageContent{
			{
				Role: llms.ChatMessageTypeSystem,
				Parts: []llms.ContentPart{
					llms.TextContent{Text: "You are a creative assistant describing futuristic scenarios."},
				},
			},
			{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.TextContent{Text: userPrompt},
				},
			},
		}

		// Generate response using GenerateContent
		resp, err := llm.GenerateContent(ctx, prompt)
		if err != nil {
			log.Fatal(err)
		}

		// Validate and extract response
		if len(resp.Choices) < 1 {
			log.Fatal("empty response from model")
		}

		fmt.Println(resp.Choices[0].Content)
	} else {
		// For non-Nova models, use GenerateFromSinglePrompt
		output, err := llms.GenerateFromSinglePrompt(ctx, llm, userPrompt)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(output)
	}
}
