package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
	util "github.com/versus-control/ai-infrastructure-agent/pkg/utilities"
)

// ========== ReAct-Style Step Recovery Engine Implementation ==========

// DefaultStepRecoveryEngine implements the StepRecoveryEngine interface
type DefaultStepRecoveryEngine struct {
	agent *StateAwareAgent
}

// NewStepRecoveryEngine creates a new step recovery engine
func NewStepRecoveryEngine(agent *StateAwareAgent) StepRecoveryEngine {
	return &DefaultStepRecoveryEngine{
		agent: agent,
	}
}

// AttemptStepRecovery tries to recover from a failed step using AI consultation
func (r *DefaultStepRecoveryEngine) AttemptStepRecovery(ctx context.Context, failureContext *StepFailureContext) (*StepRecoveryResult, error) {
	r.agent.Logger.WithFields(map[string]interface{}{
		"step_id":        failureContext.OriginalStep.ID,
		"attempt_number": failureContext.AttemptNumber,
		"error":          failureContext.FailureError,
	}).Info("Attempting ReAct-style step recovery")

	// Step 1: Analyze the failure using AI
	analysis, err := r.AnalyzeFailure(ctx, failureContext)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze step failure: %w", err)
	}

	r.agent.Logger.WithFields(map[string]interface{}{
		"recommended_action": analysis.RecommendedAction,
		"confidence":         analysis.Confidence,
		"options_count":      len(analysis.RecoveryOptions),
	}).Info("AI failure analysis completed")

	// Step 2: Select the best recovery option
	selectedOption := r.selectBestRecoveryOption(analysis.RecoveryOptions)
	if selectedOption == nil {
		return &StepRecoveryResult{
			Success:        false,
			AttemptNumber:  failureContext.AttemptNumber,
			RecoveryAction: "fail_plan",
			Reasoning:      "No viable recovery options found",
		}, nil
	}

	// Step 3: Validate the recovery action
	recoveryResult := &StepRecoveryResult{
		Success:         false, // Will be set based on execution
		AlternativeTool: selectedOption.ToolName,
		ModifiedParams:  selectedOption.Parameters,
		Reasoning:       selectedOption.Reasoning,
		AttemptNumber:   failureContext.AttemptNumber,
		RecoveryAction:  selectedOption.Action,
	}

	if err := r.ValidateRecoveryAction(ctx, recoveryResult, failureContext); err != nil {
		r.agent.Logger.WithError(err).Warn("Recovery action validation failed")
		return &StepRecoveryResult{
			Success:        false,
			AttemptNumber:  failureContext.AttemptNumber,
			RecoveryAction: "fail_plan",
			Reasoning:      fmt.Sprintf("Recovery validation failed: %v", err),
		}, nil
	}

	// Step 4: Execute the recovery action
	return r.executeRecoveryAction(ctx, recoveryResult, failureContext)
}

// AnalyzeFailure analyzes the failure and suggests recovery options using AI
func (r *DefaultStepRecoveryEngine) AnalyzeFailure(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error) {
	// Build comprehensive prompt for AI analysis
	prompt := r.buildRecoveryAnalysisPrompt(failureContext)

	r.agent.Logger.WithField("prompt_length", len(prompt)).Debug("Built recovery analysis prompt")

	// Call AI model for analysis using GenerateContent
	messages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: "You are an expert infrastructure recovery agent. Analyze failures and provide structured recovery options in JSON format."},
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: prompt},
			},
		},
	}

	resp, err := r.agent.llm.GenerateContent(ctx, messages,
		llms.WithTemperature(0.1),
		llms.WithMaxTokens(r.agent.config.MaxTokens))
	if err != nil {
		return nil, fmt.Errorf("AI consultation failed: %w", err)
	}

	// Extract response content
	var content string
	if len(resp.Choices) > 0 && len(resp.Choices[0].Content) > 0 {
		content = resp.Choices[0].Content
	} else {
		return nil, fmt.Errorf("empty response from AI model")
	}

	// Log the raw AI response for debugging
	r.agent.Logger.WithFields(map[string]interface{}{
		"response_length": len(content),
		"raw_response":    content,
	}).Debug("Raw AI recovery analysis response")

	// Parse the AI response into structured analysis
	analysis, err := r.parseRecoveryAnalysis(content)
	if err != nil {
		r.agent.Logger.WithError(err).WithField("raw_response", content).Warn("Failed to parse AI recovery analysis, using fallback")
		// Fallback to basic analysis
		return r.createFallbackAnalysis(failureContext), nil
	}

	return analysis, nil
}

// GetSimilarTools finds alternative tools that might achieve the same goal
func (r *DefaultStepRecoveryEngine) GetSimilarTools(ctx context.Context, originalTool string, objective string) ([]MCPToolInfo, error) {
	r.agent.capabilityMutex.RLock()
	defer r.agent.capabilityMutex.RUnlock()

	var similarTools []MCPToolInfo

	// Extract resource type from original tool
	resourceType, _ := r.parseToolComponents(originalTool)

	// Find tools that work with the same resource type or achieve similar objectives
	for toolName, toolInfo := range r.agent.mcpTools {
		if toolName == originalTool {
			continue // Skip the original tool
		}

		// Check if tool works with same resource type
		if r.matchesResourceType(toolName, resourceType) {
			similarTools = append(similarTools, toolInfo)
			continue
		}

		// Check if tool description matches the objective
		if r.matchesObjective(toolInfo.Description, objective) {
			similarTools = append(similarTools, toolInfo)
		}
	}

	r.agent.Logger.WithFields(map[string]interface{}{
		"original_tool": originalTool,
		"objective":     objective,
		"similar_tools": len(similarTools),
	}).Debug("Found similar tools for recovery")

	return similarTools, nil
}

// ValidateRecoveryAction ensures the proposed recovery action is safe and feasible
func (r *DefaultStepRecoveryEngine) ValidateRecoveryAction(ctx context.Context, action *StepRecoveryResult, context *StepFailureContext) error {
	switch action.RecoveryAction {
	case "retry_same":
		// Validate that we haven't exceeded retry limits
		if context.AttemptNumber >= 3 {
			return fmt.Errorf("maximum retry attempts reached")
		}
		return nil

	case "try_alternative":
		// Validate that the alternative tool exists and is available
		if action.AlternativeTool == "" {
			return fmt.Errorf("no alternative tool specified")
		}

		r.agent.capabilityMutex.RLock()
		toolInfo, exists := r.agent.mcpTools[action.AlternativeTool]
		r.agent.capabilityMutex.RUnlock()

		if !exists {
			return fmt.Errorf("alternative tool %s not available", action.AlternativeTool)
		}

		// Validate parameters against tool schema
		if err := r.validateToolParameters(action.ModifiedParams, toolInfo.InputSchema); err != nil {
			return fmt.Errorf("invalid parameters for alternative tool: %w", err)
		}
		return nil

	case "modify_params":
		// Validate that parameter modifications are safe
		return r.validateParameterModifications(action.ModifiedParams, context)

	case "skip_step":
		// Validate that skipping this step won't break dependencies
		return r.validateStepSkip(context)

	case "fail_plan":
		// Always valid - represents controlled failure
		return nil

	default:
		return fmt.Errorf("unknown recovery action: %s", action.RecoveryAction)
	}
}

// ========== Private Helper Methods ==========

// buildRecoveryAnalysisPrompt creates a comprehensive prompt for AI analysis
func (r *DefaultStepRecoveryEngine) buildRecoveryAnalysisPrompt(failureContext *StepFailureContext) string {
	var prompt strings.Builder

	prompt.WriteString("# ReAct-Style Step Recovery Analysis\n\n")
	prompt.WriteString(fmt.Sprintf("🚨 **CRITICAL STEP ID FORMAT**: The failing step ID is '%s'. Recovery steps will have IDs: %s-recovery-1, %s-recovery-2, etc.\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString(fmt.Sprintf("🚨 **DEPENDENCY REFERENCE RULE**: Use ONLY {{%s-recovery-1.field}}, {{%s-recovery-2.field}}, etc. NEVER invent step names!\n\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString("You are an intelligent infrastructure agent tasked with recovering from a failed execution step. ")
	prompt.WriteString("Analyze the failure and provide structured recovery options. ")
	prompt.WriteString("**IMPORTANT**: For data dependency failures (like missing subnet IDs, VPC IDs, etc.), ")
	prompt.WriteString("provide multi-step recovery where the first step gathers the required data and subsequent steps ")
	prompt.WriteString("use that data to retry the original operation.\n\n")

	// Step failure information
	prompt.WriteString("## Failed Step Information\n")
	prompt.WriteString(fmt.Sprintf("- **Step ID**: %s\n", failureContext.OriginalStep.ID))
	prompt.WriteString(fmt.Sprintf("- **Step Name**: %s\n", failureContext.OriginalStep.Name))
	prompt.WriteString(fmt.Sprintf("- **Action**: %s\n", failureContext.OriginalStep.Action))
	prompt.WriteString(fmt.Sprintf("- **Tool Used**: %s\n", failureContext.OriginalStep.MCPTool))
	prompt.WriteString(fmt.Sprintf("- **Error**: %s\n", failureContext.FailureError))
	prompt.WriteString(fmt.Sprintf("- **Attempt Number**: %d\n\n", failureContext.AttemptNumber))

	// Current step's parameters and tool parameters for analysis
	prompt.WriteString("## Current Step Configuration\n")
	if len(failureContext.OriginalStep.Parameters) > 0 {
		prompt.WriteString("**Step Parameters**:\n")
		for key, value := range failureContext.OriginalStep.Parameters {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
	}
	if len(failureContext.OriginalStep.ToolParameters) > 0 {
		prompt.WriteString("**Tool Parameters**:\n")
		for key, value := range failureContext.OriginalStep.ToolParameters {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
	}
	prompt.WriteString("\n")

	// Previous attempts
	if len(failureContext.PreviousAttempts) > 0 {
		prompt.WriteString("## Previous Recovery Attempts\n")
		for _, attempt := range failureContext.PreviousAttempts {
			prompt.WriteString(fmt.Sprintf("- **Attempt %d**: Used %s, Result: %s\n",
				attempt.AttemptNumber, attempt.ToolUsed, attempt.Error))
		}
		prompt.WriteString("\n")
	}

	// Available tools with detailed information
	prompt.WriteString("## Available Tools for Recovery\n")
	prompt.WriteString("**CRITICAL**: Choose recovery tools ONLY from this list. Do not assume or hardcode tool names.\n")
	prompt.WriteString("Analyze tool descriptions to find appropriate discovery/listing tools for missing resources:\n\n")
	for _, tool := range failureContext.AvailableTools {
		prompt.WriteString(fmt.Sprintf("### %s\n", tool.Name))
		prompt.WriteString(fmt.Sprintf("- **Description**: %s\n", tool.Description))
		if tool.InputSchema != nil {
			if properties, exists := tool.InputSchema["properties"].(map[string]interface{}); exists {
				prompt.WriteString("- **Parameters**: ")
				var params []string
				for paramName := range properties {
					params = append(params, paramName)
				}
				prompt.WriteString(fmt.Sprintf("%s\n", strings.Join(params, ", ")))
			}
		}
		prompt.WriteString("\n")
	}

	// Current execution context
	prompt.WriteString("## Execution Context\n")
	prompt.WriteString(fmt.Sprintf("- **Completed Steps**: %d\n", len(failureContext.CompletedSteps)))
	prompt.WriteString(fmt.Sprintf("- **Remaining Steps**: %d\n", len(failureContext.RemainingSteps)))
	prompt.WriteString(fmt.Sprintf("- **AWS Region**: %s\n\n", failureContext.AWSRegion))

	// Add dynamic infrastructure state information
	if failureContext.CurrentState != nil {
		r.addInfrastructureContextSection(&prompt, failureContext)
	}

	// Add completed steps output context for referencing
	if len(failureContext.CompletedSteps) > 0 {
		r.addCompletedStepsContext(&prompt, failureContext)
	}

	// Request structured response
	prompt.WriteString("## Required Response Format\n")
	prompt.WriteString("Provide your analysis in the following JSON format. ")
	prompt.WriteString("**IMPORTANT**: Use only valid JSON syntax without comments or explanatory text within the JSON structure.\n\n")
	prompt.WriteString("```json\n")
	prompt.WriteString(`{
  "failure_reason": "Root cause analysis of why the step failed",
  "recovery_options": [
    {
      "action": "multi_step_recovery|retry_same|try_alternative|modify_params|skip_step",
      "tool_name": "tool-for-single-step-recovery-or-first-tool-in-multi-step",
      "parameters": {"param1": "value1"},
      "reasoning": "Why this approach should work",
      "success_probability": 0.8,
      "risk_level": "low|medium|high",
      "dependencies": ["step-ids-that-must-complete-first"],
      "multi_step_plan": [
        {
          "step_order": 1,
          "tool_name": "select-from-available-tools",
          "parameters": {"region": "{{context.region}}", "filter": "appropriate-filter"},
          "purpose": "Discover required resources based on failure analysis"
        },
        {
          "step_order": 2,
          "tool_name": "original-failing-tool-or-alternative",
          "parameters": {"required_param": "{{original-step-id-recovery-1.actual_field}}"},
          "purpose": "Complete the original operation with discovered parameters"
        }
      ]
    }
  ],
  "recommended_action": "multi_step_recovery|retry_same|try_alternative|modify_params|skip_step|fail_plan",
  "confidence": 0.85,
  "requires_user_input": false,
  "risk_assessment": "Overall risk assessment",
  "alternative_approach": "If complete replanning is needed"
}`)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("Focus on practical, safe recovery options. Consider service limitations, ")
	prompt.WriteString("resource dependencies, and infrastructure consistency. Prioritize solutions with ")
	prompt.WriteString("high success probability and low risk.\n\n")

	// Add critical context resolution guidance
	prompt.WriteString("🚨 **CRITICAL: CONTEXT RESOLUTION RULES** 🚨\n")
	prompt.WriteString("**The current error likely involves `{{context.xyz}}` patterns that are NOT resolved by the system.**\n\n")
	prompt.WriteString("❌ **NEVER USE THESE PATTERNS**:\n")
	prompt.WriteString("- `{{context.vpcId}}` - System cannot resolve this\n")
	prompt.WriteString("- `{{context.subnetId}}` - System cannot resolve this  \n")
	prompt.WriteString("- `{{context.securityGroupId}}` - System cannot resolve this\n")
	prompt.WriteString("- Any `{{context.xyz}}` pattern - These are NOT handled by dependency resolution\n\n")
	prompt.WriteString("✅ **INSTEAD, USE**:\n")
	prompt.WriteString("- **Actual resource IDs** from 'Current Infrastructure State' section above\n")
	prompt.WriteString("- **Discovery steps** using available tools to find missing resources\n")
	prompt.WriteString("- **Step outputs** using `{{recovery-step-id.actual-field}}` format\n\n")

	prompt.WriteString("**INTELLIGENT RECOVERY STRATEGY**:\n")
	prompt.WriteString("1. **Analyze the error**: Understand what resource/parameter is missing or invalid\n")
	prompt.WriteString("2. **Review available tools**: Choose appropriate discovery/listing tools from the available tools list above\n")
	prompt.WriteString("3. **Apply infrastructure expertise**: Use your knowledge of cloud resource dependencies and relationships\n")
	prompt.WriteString("4. **Design logical discovery sequence**: Think like an expert - what resources need to be discovered and in what order?\n")
	prompt.WriteString("5. **Match tool capabilities**: Select tools that can discover the missing data based on their descriptions\n")
	prompt.WriteString("6. **Design complete multi-step recovery**: Each step should have all the parameters it needs\n")
	prompt.WriteString("7. **Use dependency references**: For step parameters that need outputs from previous recovery steps, use `{{step-id.field}}` format\n")
	prompt.WriteString("8. **Include final step**: The last step in multi_step_plan should complete the original operation with dependency references\n\n")
	prompt.WriteString("**EXPERT INFRASTRUCTURE REASONING**:\n")
	prompt.WriteString("Apply your expert knowledge to understand error patterns and resource relationships:\n\n")
	prompt.WriteString("🧠 **Error Analysis Expertise**:\n")
	prompt.WriteString("- Parse error messages to identify missing or invalid resources\n")
	prompt.WriteString("- Identify the root cause: network isolation, missing dependencies, invalid references, etc.\n")
	prompt.WriteString("- Determine what information is needed to resolve the issue\n\n")
	prompt.WriteString("🧠 **Resource Dependency Expertise**:\n")
	prompt.WriteString("- Understand cloud resource hierarchies and dependencies\n")
	prompt.WriteString("- Know that compute resources typically need: network → subnet → security → compute\n")
	prompt.WriteString("- Recognize that load balancers need: network → multiple subnets → security → load balancer\n")
	prompt.WriteString("- Understand that databases need: network → subnet groups → security → database\n\n")
	prompt.WriteString("🧠 **Tool Selection Expertise**:\n")
	prompt.WriteString("- Analyze available tool descriptions to understand their capabilities\n")
	prompt.WriteString("- Choose discovery tools that can find the missing resources\n")
	prompt.WriteString("- Understand tool output schemas to extract correct field names\n")
	prompt.WriteString("- Select tools that work together in a logical sequence\n\n")
	prompt.WriteString("🧠 **Field Mapping Expertise**:\n")
	prompt.WriteString("- Analyze tool response schemas to understand output field names\n")
	prompt.WriteString("- Distinguish between different resource types (network IDs vs subnet IDs vs security group IDs)\n")
	prompt.WriteString("- Map the correct resource identifiers to the parameters that need them\n")
	prompt.WriteString("- Ensure resource references match what the target operation expects\n\n")
	prompt.WriteString("**MULTI-STEP RECOVERY PRINCIPLES**:\n")
	prompt.WriteString("- Design a complete sequence where each step has all necessary parameters\n")
	prompt.WriteString("- The final step should be the corrected version of the original failing step\n")
	prompt.WriteString("- **Use dependency references**: When a step needs output from a previous recovery step, use `{{recovery-step-id.field}}` format\n")
	prompt.WriteString(fmt.Sprintf("- **CRITICAL**: Recovery step IDs are: %s-recovery-1, %s-recovery-2, etc. (NOT tool names!)\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString("- **Dynamic field references**: Choose appropriate output fields based on the actual tool response structure\n")
	prompt.WriteString("- **Parameter substitution**: The system will automatically substitute dependency references with actual values from previous step outputs\n\n")
	prompt.WriteString("**DEPENDENCY REFERENCE REQUIREMENTS**:\n")
	prompt.WriteString("🚨 NEVER use hardcoded AWS resource IDs like subnet-12345678, vpc-abcdef, sg-987654, etc.\n")
	prompt.WriteString("🚨 NEVER hardcode specific field names - analyze actual tool output structure\n")
	prompt.WriteString(fmt.Sprintf("✅ CRITICAL: Recovery step IDs follow this EXACT pattern: %s-recovery-1, %s-recovery-2, %s-recovery-3, etc.\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString(fmt.Sprintf("✅ USE EXACT FORMAT: {{%s-recovery-1.fieldName}}, {{%s-recovery-2.fieldName}}, etc.\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString("✅ ONLY reference recovery steps that you define in your multi_step_plan\n")
	prompt.WriteString("✅ CHOOSE APPROPRIATE TOOLS: Select from available tools list, don't assume specific tool names\n")
	prompt.WriteString("✅ ANALYZE TOOL OUTPUTS: Use actual field names from tool response schemas\n\n")
	prompt.WriteString("**GENERIC RECOVERY PATTERN EXAMPLE**:\n")
	prompt.WriteString(fmt.Sprintf("For failed step '%s' requiring resource discovery and retry:\n", failureContext.OriginalStep.ID))
	prompt.WriteString("```json\n")
	prompt.WriteString(`"multi_step_plan": [
  {
    "step_order": 1,
    "tool_name": "appropriate-discovery-tool-from-available-list",
    "parameters": {"region": "{{context.region}}"},
    "purpose": "Discover required resources based on failure context"
  },
  {
    "step_order": 2,
    "tool_name": "original-failing-tool-or-alternative",
    "parameters": {"required_param": "{{`)
	prompt.WriteString(fmt.Sprintf("%s-recovery-1", failureContext.OriginalStep.ID))
	prompt.WriteString(`.actual_field_name}}"},
    "purpose": "Retry original operation with discovered resource data"
  }
]`)
	prompt.WriteString("\n```\n\n")
	prompt.WriteString("**EXPERT RECOVERY REASONING EXAMPLES**:\n")
	prompt.WriteString("Think through these scenarios using infrastructure expertise:\n\n")
	prompt.WriteString("🔄 **Network Dependency Resolution**:\n")
	prompt.WriteString("If error indicates missing network context (no VPC, invalid subnet, etc.):\n")
	prompt.WriteString("```json\n")
	prompt.WriteString(`{
  "reasoning": "Error indicates network isolation - need to discover network topology",
  "multi_step_plan": [
    {
      "step_order": 1,
      "tool_name": "[select appropriate network discovery tool from available tools]",
      "parameters": {"region": "{{context.region}}", "filters": {"state": "available"}},
      "purpose": "Discover available network infrastructure"
    },
    {
      "step_order": 2,
      "tool_name": "[select subnet/subnetwork discovery tool]",
      "parameters": {"parent_network": "{{`)
	prompt.WriteString(fmt.Sprintf("%s-recovery-1", failureContext.OriginalStep.ID))
	prompt.WriteString(`.[network_identifier_field]}}", "region": "{{context.region}}"},
      "purpose": "Find subnets within the discovered network"
    },
    {
      "step_order": 3,
      "tool_name": "[original failing tool]",
      "parameters": {
        "subnet_parameter": "{{`)
	prompt.WriteString(fmt.Sprintf("%s-recovery-2", failureContext.OriginalStep.ID))
	prompt.WriteString(`.[subnet_identifier_field]}}",
        "[preserve other original parameters]": "{{context.original_params}}"
      },
      "purpose": "Retry operation with proper network context"
    }
  ]
}`)
	prompt.WriteString("\n```\n\n")
	prompt.WriteString("🔄 **Resource Reference Resolution**:\n")
	prompt.WriteString("If error indicates invalid resource reference:\n")
	prompt.WriteString("```json\n")
	prompt.WriteString(`{
  "reasoning": "Error shows invalid resource ID - need to discover valid resources of this type",
  "multi_step_plan": [
    {
      "step_order": 1,
      "tool_name": "[select appropriate resource discovery tool]",
      "parameters": {"region": "{{context.region}}", "resource_type": "[infer from error]"},
      "purpose": "Find valid resources of the required type"
    },
    {
      "step_order": 2,
      "tool_name": "[original failing tool]",
      "parameters": {
        "[failing_parameter]": "{{`)
	prompt.WriteString(fmt.Sprintf("%s-recovery-1", failureContext.OriginalStep.ID))
	prompt.WriteString(`.[appropriate_resource_field]}}",
        "[other_parameters]": "{{context.preserve_other_params}}"
      },
      "purpose": "Retry with valid resource reference"
    }
  ]
}`)
	prompt.WriteString("\n```\n\n")
	prompt.WriteString("**CRITICAL**: Return ONLY valid JSON without any comments (//), explanations, or additional text. ")
	prompt.WriteString("Do not include inline comments like '// comment here' within the JSON structure.\n\n")
	prompt.WriteString("**EXPERT VALIDATION FRAMEWORK**:\n")
	prompt.WriteString("Apply infrastructure expertise to validate your recovery plan:\n\n")
	prompt.WriteString("🎯 **Error Root Cause Analysis**:\n")
	prompt.WriteString("- Does the error indicate missing resources, invalid references, or configuration issues?\n")
	prompt.WriteString("- What type of resource discovery or validation is needed to resolve this specific error?\n")
	prompt.WriteString("- Are there dependency relationships that need to be established?\n\n")
	prompt.WriteString("🎯 **Resource Relationship Validation**:\n")
	prompt.WriteString("- Do your discovery steps follow logical infrastructure dependencies?\n")
	prompt.WriteString("- Are you discovering parent resources before child resources?\n")
	prompt.WriteString("- Are you using the correct resource identifiers for each parameter?\n\n")
	prompt.WriteString("🎯 **Tool Selection Validation**:\n")
	prompt.WriteString("- Are the selected tools available in the provided tools list?\n")
	prompt.WriteString("- Do the tool descriptions match what you're trying to accomplish?\n")
	prompt.WriteString("- Can each tool provide the output fields you're referencing?\n\n")
	prompt.WriteString("🎯 **Parameter Mapping Validation**:\n")
	prompt.WriteString("- Are you using the correct field names from tool outputs?\n")
	prompt.WriteString("- Are resource types properly matched (network IDs for network params, subnet IDs for subnet params)?\n")
	prompt.WriteString("- Are dependency references correctly formatted and logically sequenced?\n\n")
	prompt.WriteString("🎯 **Final Operation Validation**:\n")
	prompt.WriteString("- Does the final step retry the original operation with all necessary parameters?\n")
	prompt.WriteString("- Are all missing or invalid parameters now provided from discovery steps?\n")
	prompt.WriteString("- Will this sequence address the root cause of the original failure?\n\n")
	prompt.WriteString("**DEPENDENCY REFERENCE FORMAT RULES**:\n")
	prompt.WriteString(fmt.Sprintf("- Use exact format: `{{%s-recovery-N.field_name}}`\n", failureContext.OriginalStep.ID))
	prompt.WriteString(fmt.Sprintf("- For this failing step '%s': recovery step IDs are %s-recovery-1, %s-recovery-2, etc.\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString("- Replace 'N' with recovery step number (1, 2, 3, etc.)\n")
	prompt.WriteString("- Replace 'field_name' with actual output field from tool response\n")
	prompt.WriteString(fmt.Sprintf("- Example: `{{%s-recovery-1.resourceId}}`, `{{%s-recovery-2.subnetId}}`\n", failureContext.OriginalStep.ID, failureContext.OriginalStep.ID))
	prompt.WriteString("- NEVER use hardcoded field names - analyze actual tool output structure\n")
	prompt.WriteString("- CRITICAL: In step 2, reference step 1 as " + fmt.Sprintf(`{{%s-recovery-1.field}}`, failureContext.OriginalStep.ID) + "\n")
	prompt.WriteString("- CRITICAL: In step 3, reference step 1 as " + fmt.Sprintf(`{{%s-recovery-1.field}}`, failureContext.OriginalStep.ID) + " or step 2 as " + fmt.Sprintf(`{{%s-recovery-2.field}}`, failureContext.OriginalStep.ID) + "\n\n")
	prompt.WriteString("**EXPERT REASONING CHECKLIST**:\n")
	prompt.WriteString("Before submitting your recovery plan, apply expert infrastructure knowledge:\n\n")
	prompt.WriteString("✅ **Error Understanding**: Have I correctly identified what resource or parameter is missing/invalid?\n")
	prompt.WriteString("✅ **Resource Logic**: Does my discovery sequence follow logical infrastructure dependencies?\n")
	prompt.WriteString("✅ **Tool Analysis**: Have I selected tools based on their actual descriptions and capabilities?\n")
	prompt.WriteString("✅ **Field Expertise**: Am I using the correct resource identifiers for each parameter type?\n")
	prompt.WriteString("✅ **Dependency Flow**: Do my reference chains logically connect discovery outputs to final operation inputs?\n")
	prompt.WriteString("✅ **Root Cause Resolution**: Will this sequence actually resolve the original error condition?\n\n")
	prompt.WriteString("**INTELLIGENT ADAPTATION**:\n")
	prompt.WriteString("Remember: You are an expert infrastructure consultant. Apply your knowledge dynamically:\n")
	prompt.WriteString("- Analyze error messages to understand the specific failure mode\n")
	prompt.WriteString("- Consider infrastructure patterns and resource relationships\n")
	prompt.WriteString("- Choose tools and sequences that make logical sense for the specific scenario\n")
	prompt.WriteString("- Don't follow rigid patterns - adapt based on the actual error and available tools\n")
	prompt.WriteString("- Think through the complete resolution path from error to working state\n")

	return prompt.String()
}

// parseRecoveryAnalysis parses the AI response into structured analysis
func (r *DefaultStepRecoveryEngine) parseRecoveryAnalysis(response string) (*AIRecoveryAnalysis, error) {
	// Use existing agent JSON processing methods instead of custom implementation
	var jsonStr string

	// Try primary extraction method
	jsonStr = r.agent.extractJSON(response)

	// If primary method fails, try alternative methods
	if jsonStr == "" {
		jsonStr = r.agent.extractJSONAlternative(response)
	}

	// If still no JSON found, try truncated JSON parsing
	if jsonStr == "" {
		jsonStr = r.agent.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	// Clean up common AI JSON issues (like inline comments)
	jsonStr = r.agent.cleanJSONComments(jsonStr)

	if r.agent.config.EnableDebug {
		r.agent.Logger.WithFields(map[string]interface{}{
			"extracted_json": jsonStr,
			"json_length":    len(jsonStr),
		}).Debug("Extracted JSON from AI response using agent JSON processing")
	}

	var analysis AIRecoveryAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate the parsed analysis
	if len(analysis.RecoveryOptions) == 0 {
		return nil, fmt.Errorf("no recovery options provided by AI")
	}

	return &analysis, nil
}

// createFallbackAnalysis creates a basic analysis when AI parsing fails
func (r *DefaultStepRecoveryEngine) createFallbackAnalysis(failureContext *StepFailureContext) *AIRecoveryAnalysis {
	options := []*RecoveryOption{}

	// Basic retry option
	if failureContext.AttemptNumber < 3 {
		options = append(options, &RecoveryOption{
			Action:             "retry_same",
			Reasoning:          "Retry the same operation - may be a transient failure",
			SuccessProbability: 0.5,
			RiskLevel:          "low",
		})
	}

	// Look for similar tools
	similarTools, _ := r.GetSimilarTools(context.Background(), failureContext.OriginalStep.MCPTool, failureContext.OriginalStep.Description)
	if len(similarTools) > 0 {
		options = append(options, &RecoveryOption{
			Action:             "try_alternative",
			ToolName:           similarTools[0].Name,
			Parameters:         failureContext.OriginalStep.Parameters,
			Reasoning:          fmt.Sprintf("Try alternative tool: %s", similarTools[0].Name),
			SuccessProbability: 0.6,
			RiskLevel:          "medium",
		})
	}

	return &AIRecoveryAnalysis{
		FailureReason:     "Step execution failed",
		RecoveryOptions:   options,
		RecommendedAction: "retry_same",
		Confidence:        0.5,
		RequiresUserInput: false,
		RiskAssessment:    "Low risk fallback analysis",
	}
}

// selectBestRecoveryOption selects the best recovery option from AI suggestions
func (r *DefaultStepRecoveryEngine) selectBestRecoveryOption(options []*RecoveryOption) *RecoveryOption {
	if len(options) == 0 {
		return nil
	}

	// Score each option based on success probability, risk level, and action type
	bestOption := options[0]
	bestScore := r.calculateOptionScore(bestOption)

	for _, option := range options[1:] {
		score := r.calculateOptionScore(option)
		if score > bestScore {
			bestScore = score
			bestOption = option
		}
	}

	return bestOption
}

// calculateOptionScore scores a recovery option for selection
func (r *DefaultStepRecoveryEngine) calculateOptionScore(option *RecoveryOption) float64 {
	score := option.SuccessProbability

	// Adjust for risk level
	switch option.RiskLevel {
	case "low":
		score += 0.1
	case "medium":
		score += 0.0
	case "high":
		score -= 0.2
	}

	// Prefer certain action types
	switch option.Action {
	case "retry_same":
		score += 0.05
	case "try_alternative":
		score += 0.1
	case "modify_params":
		score += 0.0
	case "skip_step":
		score -= 0.1
	}

	return score
}

// executeRecoveryAction executes the selected recovery action
func (r *DefaultStepRecoveryEngine) executeRecoveryAction(ctx context.Context, action *StepRecoveryResult, failureContext *StepFailureContext) (*StepRecoveryResult, error) {
	startTime := time.Now()

	switch action.RecoveryAction {
	case "retry_same":
		// Retry the original step with same parameters
		result, err := r.executeStepAction(failureContext.OriginalStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Retry failed: %v", err)
		} else {
			action.Output = result
		}

	case "try_alternative":
		// Create modified step with alternative tool
		modifiedStep := *failureContext.OriginalStep
		modifiedStep.MCPTool = action.AlternativeTool
		if action.ModifiedParams != nil {
			modifiedStep.Parameters = action.ModifiedParams
		}

		result, err := r.executeStepAction(&modifiedStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Alternative tool failed: %v", err)
		} else {
			action.Output = result
		}

	case "modify_params":
		// Retry with modified parameters
		modifiedStep := *failureContext.OriginalStep
		modifiedStep.Parameters = action.ModifiedParams

		result, err := r.executeStepAction(&modifiedStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Modified parameters failed: %v", err)
		} else {
			action.Output = result
		}

	case "skip_step":
		// Mark as successful skip
		action.Success = true
		action.Output = map[string]interface{}{"skipped": true}

	case "fail_plan":
		// Controlled failure
		action.Success = false
		action.Reasoning = "Recovery determined plan should fail"

	default:
		return nil, fmt.Errorf("unknown recovery action: %s", action.RecoveryAction)
	}

	duration := time.Since(startTime)
	r.agent.Logger.WithFields(map[string]interface{}{
		"recovery_action": action.RecoveryAction,
		"success":         action.Success,
		"duration":        duration,
	}).Info("Recovery action completed")

	return action, nil
}

// executeStepAction executes a step action (helper method)
func (r *DefaultStepRecoveryEngine) executeStepAction(step *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Resolve any template parameters first
	resolvedParams := make(map[string]interface{})
	for key, value := range step.Parameters {
		if strValue, ok := value.(string); ok && strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
			if resolvedValue, err := r.agent.resolveDependencyReference(strValue); err == nil {
				resolvedParams[key] = resolvedValue
			} else {
				resolvedParams[key] = value // Fallback to original
			}
		} else {
			resolvedParams[key] = value
		}
	}

	// Execute the MCP tool call
	result, err := r.agent.callMCPTool(step.MCPTool, resolvedParams)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ========== Validation Helper Methods ==========

func (r *DefaultStepRecoveryEngine) parseToolComponents(toolName string) (resourceType, action string) {
	// Parse tool name like "create-vpc" -> resource="vpc", action="create"
	parts := strings.Split(toolName, "-")
	if len(parts) >= 2 {
		action = parts[0]
		resourceType = strings.Join(parts[1:], "-")
	}
	return
}

func (r *DefaultStepRecoveryEngine) matchesResourceType(toolName, resourceType string) bool {
	return strings.Contains(toolName, resourceType)
}

func (r *DefaultStepRecoveryEngine) matchesObjective(description, objective string) bool {
	// Simple keyword matching - could be enhanced with semantic similarity
	descLower := strings.ToLower(description)
	objLower := strings.ToLower(objective)

	words := strings.Fields(objLower)
	matches := 0

	for _, word := range words {
		if strings.Contains(descLower, word) {
			matches++
		}
	}

	// Consider a match if at least 30% of words match
	return float64(matches)/float64(len(words)) >= 0.3
}

func (r *DefaultStepRecoveryEngine) validateToolParameters(params map[string]interface{}, schema map[string]interface{}) error {
	// Basic parameter validation - could be enhanced with JSON schema validation
	// For now, just check required parameters exist

	_, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil // No validation rules
	}

	required, ok := schema["required"].([]interface{})
	if !ok {
		return nil // No required parameters
	}

	for _, reqParam := range required {
		paramName, ok := reqParam.(string)
		if !ok {
			continue
		}

		if _, exists := params[paramName]; !exists {
			return fmt.Errorf("required parameter %s is missing", paramName)
		}
	}

	return nil
}

func (r *DefaultStepRecoveryEngine) validateParameterModifications(params map[string]interface{}, context *StepFailureContext) error {
	// Validate that parameter modifications are safe
	// Check for dangerous modifications that could affect other resources

	originalParams := context.OriginalStep.Parameters

	// Check for critical parameter changes
	criticalParams := []string{"vpcId", "subnetId", "securityGroupId", "keyName"}

	for _, param := range criticalParams {
		if originalVal, origExists := originalParams[param]; origExists {
			if newVal, newExists := params[param]; newExists && originalVal != newVal {
				r.agent.Logger.WithFields(map[string]interface{}{
					"parameter":      param,
					"original_value": originalVal,
					"new_value":      newVal,
				}).Warn("Critical parameter modification detected")
			}
		}
	}

	return nil
}

func (r *DefaultStepRecoveryEngine) validateStepSkip(context *StepFailureContext) error {
	// Check if any remaining steps depend on this step's output
	skipStepID := context.OriginalStep.ID

	for _, remainingStep := range context.RemainingSteps {
		for _, param := range remainingStep.Parameters {
			if paramStr, ok := param.(string); ok {
				// Check for dependency references like {{step-create-vpc.resourceId}}
				if strings.Contains(paramStr, fmt.Sprintf("{{%s.", skipStepID)) {
					return fmt.Errorf("cannot skip step %s: step %s depends on its output", skipStepID, remainingStep.ID)
				}
			}
		}
	}

	r.agent.Logger.WithField("step_id", skipStepID).Info("Step can be safely skipped - no dependencies found")
	return nil
}

// addInfrastructureContextSection adds dynamic infrastructure state context to the prompt
func (r *DefaultStepRecoveryEngine) addInfrastructureContextSection(prompt *strings.Builder, failureContext *StepFailureContext) {
	prompt.WriteString("## Current Infrastructure State\n")
	prompt.WriteString("🚨 **CRITICAL**: Use actual resource IDs from this state instead of generic `{{context.xyz}}` patterns.\n")
	prompt.WriteString("The error likely involves invalid resource references. Use the actual IDs provided below.\n\n")

	if len(failureContext.CurrentState.Resources) == 0 {
		prompt.WriteString("⚠️ **No managed resources found** - All resources must be discovered or created from scratch.\n\n")
		return
	}

	// Organize resources by type for easier understanding
	resourcesByType := make(map[string][]*types.ResourceState)
	for _, resource := range failureContext.CurrentState.Resources {
		resourcesByType[resource.Type] = append(resourcesByType[resource.Type], resource)
	}

	// Display current resources organized by type
	for resourceType, resources := range resourcesByType {
		prompt.WriteString(fmt.Sprintf("### Available %s Resources\n", util.Title(resourceType)))
		for _, resource := range resources {
			prompt.WriteString(fmt.Sprintf("- **%s**\n", resource.Name))
			prompt.WriteString(fmt.Sprintf("  - **Resource ID**: `%s`\n", resource.ID))
			prompt.WriteString(fmt.Sprintf("  - **Status**: %s\n", resource.Status))

			// Include important properties that are commonly referenced
			if resource.Properties != nil {
				for propKey, propValue := range resource.Properties {
					switch propKey {
					case "VpcId", "SubnetId", "SecurityGroupId", "AvailabilityZone", "CidrBlock", "ImageId", "InstanceType":
						prompt.WriteString(fmt.Sprintf("  - **%s**: `%v`\n", propKey, propValue))
					}
				}
			}
		}
		prompt.WriteString("\n")
	}

	// Provide concrete examples based on actual infrastructure
	prompt.WriteString("### 🎯 Correct Resource References\n")
	prompt.WriteString("**Instead of using `{{context.xyz}}` patterns, use these actual resource IDs**:\n")

	for resourceType, resources := range resourcesByType {
		if len(resources) > 0 {
			// Dynamic resource type display (capitalize first letter)
			displayName := util.Title(strings.ReplaceAll(resourceType, "-", " "))
			contextPattern := fmt.Sprintf("{{context.%sId}}", strings.ReplaceAll(resourceType, "-", ""))

			// Show up to 3 resources of each type
			maxShow := 3
			if len(resources) < maxShow {
				maxShow = len(resources)
			}

			for i := 0; i < maxShow; i++ {
				resource := resources[i]
				if len(resources) == 1 {
					// Single resource
					prompt.WriteString(fmt.Sprintf("- **%s**: Use `%s` (not `%s`)\n", displayName, resource.ID, contextPattern))
				} else {
					// Multiple resources - show with index
					prompt.WriteString(fmt.Sprintf("- **%s %d**: Use `%s` (not `%s`)\n", displayName, i+1, resource.ID, contextPattern))
				}
			}

			// Show count if there are more resources
			if len(resources) > maxShow {
				prompt.WriteString(fmt.Sprintf("- **...and %d more %s resources**\n", len(resources)-maxShow, displayName))
			}
		}
	}
	prompt.WriteString("\n")

	// Add resource discovery guidance
	prompt.WriteString("### 📋 Resource Discovery Strategy\n")
	prompt.WriteString("If you need additional resources not listed above:\n")
	prompt.WriteString("1. **Use discovery tools** from the Available Tools list to find existing resources\n")
	prompt.WriteString("2. **Reference discovered resources** using `{{recovery-step-id.actual-field-name}}` format\n")
	prompt.WriteString("3. **Never use** `{{context.xyz}}` patterns - they are not resolved by the system\n\n")
}

// addCompletedStepsContext adds context about completed steps and their outputs
func (r *DefaultStepRecoveryEngine) addCompletedStepsContext(prompt *strings.Builder, failureContext *StepFailureContext) {
	prompt.WriteString("## Completed Steps Output Context\n")
	prompt.WriteString("**Available for reference in recovery steps using `{{step-id.field}}` format**:\n\n")

	for _, completedStep := range failureContext.CompletedSteps {
		if len(completedStep.Output) > 0 {
			prompt.WriteString(fmt.Sprintf("### %s (ID: `%s`)\n", completedStep.Name, completedStep.ID))
			prompt.WriteString("**Available output fields**:\n")

			// Show all output fields that might be useful
			for key, value := range completedStep.Output {
				// Skip internal/system fields
				if !util.IsInternalField(key) {
					prompt.WriteString(fmt.Sprintf("- `{{%s.%s}}` = `%v`\n", completedStep.ID, key, value))
				}
			}
			prompt.WriteString("\n")
		}
	}

	// Add guidance on using step outputs
	prompt.WriteString("### 📖 Step Output Reference Guide\n")
	prompt.WriteString("- Use `{{step-id.field-name}}` to reference outputs from completed steps\n")
	prompt.WriteString("- Use `{{recovery-step-id.field-name}}` to reference outputs from recovery steps\n")
	prompt.WriteString("- Field names must match the actual output structure shown above\n\n")
}
