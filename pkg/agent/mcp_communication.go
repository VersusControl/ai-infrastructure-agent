package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
	util "github.com/versus-control/ai-infrastructure-agent/pkg/utilities"
)

// ========== Interface defines ==========

// MCPCommunicationInterface defines MCP (Model Context Protocol) communication functionality
//
// Available Functions:
//   - startMCPProcess()                 : Start the MCP server subprocess for tool execution
//   - stopMCPProcess()                  : Stop the MCP server process
//   - initializeMCP()                   : Initialize MCP connection and handshake
//   - ensureMCPCapabilities()           : Ensure MCP capabilities are discovered and available
//   - sendMCPRequest()                  : Send JSON-RPC request to MCP server
//   - sendMCPNotification()             : Send notification to MCP server
//   - discoverMCPCapabilities()         : Discover available tools and resources from MCP server
//   - logDiscoveredCapabilities()       : Log all discovered tools and resources for debugging
//   - discoverMCPTools()                : Discover available tools from the MCP server
//   - discoverMCPResources()            : Discover available resources from the MCP server
//   - callMCPTool()                     : Call a tool via the MCP server
//   - getStringFromMap()                : Helper function to safely extract string from map
//
//   - AnalyzeInfrastructureState()      : Call MCP server to analyze infrastructure state
//   - DetectInfrastructureConflicts()   : Call MCP server to detect conflicts
//   - PlanInfrastructureDeployment()    : Call MCP server to plan deployment
//   - VisualizeDependencyGraph()        : Call MCP server to visualize dependency graph
//   - ExportInfrastructureState()       : Call MCP server to export infrastructure state
//   - ExportInfrastructureStateWithOptions() : Export state with full control options
//   - AddResourceToState()              : Add resource to state via MCP server
//
// Usage Example:
//   1. agent.startMCPProcess()
//   2. state, resources, conflicts := agent.AnalyzeInfrastructureState(ctx, true)
//   3. deploymentOrder := agent.PlanInfrastructureDeployment(ctx, nil, false)

// ========== MCP Process Management ==========

// startMCPProcess starts the MCP server process for tool execution
func (a *StateAwareAgent) startMCPProcess() error {
	if a.mcpProcess != nil {
		return nil // Already started
	}

	a.Logger.Info("Starting MCP server process for tool execution")

	// Start the MCP server as a subprocess
	var cmd *exec.Cmd
	if _, err := os.Stat("/app/server"); err == nil {
		// Use binary in Docker container path
		cmd = exec.Command("/app/server")
	} else {
		// Development fallback - use go run
		cmd = exec.Command("go", "run", "cmd/server/main.go")
		cmd.Dir = "." // Current directory should be the project root
	}

	// Set environment variables from config
	envVars := append(os.Environ(),
		fmt.Sprintf("AWS_REGION=%s", a.awsConfig.Region),
	)

	cmd.Env = envVars

	// Setup pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Redirect MCP server stderr to our logger so we can see debug output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Start a goroutine to read stderr and log it
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			a.Logger.WithField("source", "mcp_server").Info(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			a.Logger.WithError(err).Warn("Error reading MCP server stderr")
		}
	}()

	// Create scanner with increased buffer size for large responses
	scanner := bufio.NewScanner(stdout)
	// Set max buffer size to 1MB to handle large infrastructure state responses
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	a.mcpProcess = &MCPProcess{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: scanner,
		mutex:  sync.Mutex{},
		reqID:  0,
	}

	// Initialize MCP connection
	if err := a.initializeMCP(); err != nil {
		a.stopMCPProcess()
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	a.Logger.Info("MCP server process started successfully")
	return nil
}

// stopMCPProcess stops the MCP server process
func (a *StateAwareAgent) stopMCPProcess() {
	if a.mcpProcess == nil {
		return
	}

	a.Logger.Info("Stopping MCP server process")

	if a.mcpProcess.cmd != nil && a.mcpProcess.cmd.Process != nil {
		a.mcpProcess.cmd.Process.Kill()
		a.mcpProcess.cmd.Wait()
	}

	a.mcpProcess = nil
}

// initializeMCP initializes the MCP connection
func (a *StateAwareAgent) initializeMCP() error {
	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "aws-mcp-ai-agent",
				"version": "1.0.0",
			},
		},
	}

	_, err := a.sendMCPRequest(initRequest)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Send initialized notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	}

	if err := a.sendMCPNotification(notification); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	// Discover capabilities after successful initialization
	if err := a.discoverMCPCapabilities(); err != nil {
		a.Logger.WithError(err).Error("Failed to discover MCP capabilities - this will cause tool execution to fail")
		return fmt.Errorf("failed to discover MCP capabilities: %w", err)
	}

	return nil
}

// ========== MCP Capability Discovery ==========

// ensureMCPCapabilities ensures that MCP capabilities are discovered and available
func (a *StateAwareAgent) ensureMCPCapabilities() error {
	// In test mode, skip real MCP discovery and use mock capabilities
	if a.testMode {
		return a.setupMockMCPCapabilities()
	}

	a.capabilityMutex.RLock()
	toolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	// If we have tools, capabilities are already discovered
	if toolsCount > 0 {
		return nil
	}

	a.Logger.Info("MCP capabilities not available, attempting to discover...")

	// Try to discover capabilities
	if err := a.discoverMCPCapabilities(); err != nil {
		return fmt.Errorf("failed to discover MCP capabilities: %w", err)
	}

	// Verify we now have capabilities
	a.capabilityMutex.RLock()
	finalToolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	if finalToolsCount == 0 {
		return fmt.Errorf("MCP capability discovery completed but no tools found")
	}

	a.Logger.WithField("tools_count", finalToolsCount).Info("MCP capabilities discovered successfully")
	return nil
}

// setupMockMCPCapabilities sets up mock MCP capabilities for testing
func (a *StateAwareAgent) setupMockMCPCapabilities() error {
	a.capabilityMutex.Lock()
	defer a.capabilityMutex.Unlock()

	// Create a tool factory to get the real supported tools
	factory := tools.NewToolFactory(nil, logging.NewLogger("mock", "info"))
	realToolsByAction := factory.GetSupportedToolTypes()

	// Initialize mock tools map with all real tools
	mockTools := make(map[string]MCPToolInfo)

	// Add all real tools from all action types
	for actionType, toolNames := range realToolsByAction {
		for _, toolName := range toolNames {
			mockTools[toolName] = MCPToolInfo{
				Name:        toolName,
				Description: fmt.Sprintf("Tool: %s", toolName),
				InputSchema: map[string]interface{}{"type": "object"},
				Type:        actionType,
			}

			// Register the tool's action type with the IDExtractor
			a.idExtractor.RegisterToolActionType(toolName, actionType)
		}
	}

	a.mcpTools = mockTools

	return nil
}

// ========== MCP Communication Layer ==========

// sendMCPRequest sends a request to the MCP server and waits for response
func (a *StateAwareAgent) sendMCPRequest(request map[string]interface{}) (map[string]interface{}, error) {
	a.mcpProcess.mutex.Lock()
	defer a.mcpProcess.mutex.Unlock()

	// Marshal and send request
	reqBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	a.mcpProcess.stdin.Write(reqBytes)
	a.mcpProcess.stdin.WriteString("\n")
	a.mcpProcess.stdin.Flush()

	// Read response
	if !a.mcpProcess.stdout.Scan() {
		return nil, fmt.Errorf("failed to read response")
	}

	var response map[string]interface{}
	if err := json.Unmarshal(a.mcpProcess.stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

// sendMCPNotification sends a notification to the MCP server (no response expected)
func (a *StateAwareAgent) sendMCPNotification(notification map[string]interface{}) error {
	a.mcpProcess.mutex.Lock()
	defer a.mcpProcess.mutex.Unlock()

	// Marshal and send notification
	notifBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	a.mcpProcess.stdin.Write(notifBytes)
	a.mcpProcess.stdin.WriteString("\n")
	a.mcpProcess.stdin.Flush()

	return nil
}

// discoverMCPCapabilities discovers available tools and resources from the MCP server
func (a *StateAwareAgent) discoverMCPCapabilities() error {
	a.Logger.Info("Discovering MCP server capabilities...")

	// Ensure MCP process is started
	if err := a.startMCPProcess(); err != nil {
		return fmt.Errorf("failed to start MCP process: %w", err)
	}

	// Check if MCP process is actually running
	if a.mcpProcess == nil || a.mcpProcess.cmd == nil || a.mcpProcess.cmd.Process == nil {
		return fmt.Errorf("MCP process is not running")
	}

	// Discover available tools
	if err := a.discoverMCPTools(); err != nil {
		a.Logger.WithError(err).Error("Failed to discover MCP tools")
		return fmt.Errorf("failed to discover MCP tools: %w", err)
	}

	// Discover available resources
	if err := a.discoverMCPResources(); err != nil {
		a.Logger.WithError(err).Error("Failed to discover MCP resources")
		return fmt.Errorf("failed to discover MCP resources: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"tools_count":     len(a.mcpTools),
		"resources_count": len(a.mcpResources),
	}).Info("MCP capabilities discovered successfully")

	// Verify we discovered some essential tools
	if len(a.mcpTools) == 0 {
		return fmt.Errorf("no MCP tools discovered - MCP server may not be responding correctly")
	}

	// Log all discovered tools for debugging
	a.logDiscoveredCapabilities()

	return nil
}

// logDiscoveredCapabilities logs all discovered tools and resources for debugging
func (a *StateAwareAgent) logDiscoveredCapabilities() {
	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	a.Logger.Info("=== Discovered MCP Tools ===")
	for toolName, toolInfo := range a.mcpTools {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":        toolName,
			"tool_description": toolInfo.Description,
		}).Info("Available MCP tool")
	}

	a.Logger.Info("=== Discovered MCP Resources ===")
	for uri, resourceInfo := range a.mcpResources {
		a.Logger.WithFields(map[string]interface{}{
			"resource_uri":         uri,
			"resource_name":        resourceInfo.Name,
			"resource_description": resourceInfo.Description,
		}).Info("Available MCP resource")
	}
}

// discoverMCPTools discovers available tools from the MCP server
func (a *StateAwareAgent) discoverMCPTools() error {
	a.mcpProcess.mutex.Lock()
	a.mcpProcess.reqID++
	reqID := a.mcpProcess.reqID
	a.mcpProcess.mutex.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	response, err := a.sendMCPRequest(request)
	if err != nil {
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return fmt.Errorf("MCP server error listing tools: %v", errorData)
	}

	result, exists := response["result"]
	if !exists {
		return fmt.Errorf("no result in MCP tools list response")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid result format from MCP tools list")
	}

	toolsField, exists := resultMap["tools"]
	if !exists {
		return fmt.Errorf("no tools field in MCP response")
	}

	toolsArray, ok := toolsField.([]interface{})
	if !ok {
		return fmt.Errorf("tools field is not an array")
	}

	a.capabilityMutex.Lock()
	defer a.capabilityMutex.Unlock()

	// Clear existing tools
	a.mcpTools = make(map[string]MCPToolInfo)

	// Create tool factory to determine action types
	factory := tools.NewToolFactory(nil, logging.NewLogger("tool-factory", "info"))

	for _, toolInterface := range toolsArray {
		toolMap, ok := toolInterface.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := toolMap["name"].(string)
		description, _ := toolMap["description"].(string)
		inputSchema, _ := toolMap["inputSchema"].(map[string]interface{})

		if name != "" {
			a.mcpTools[name] = MCPToolInfo{
				Name:        name,
				Description: description,
				InputSchema: inputSchema,
			}

			// Register the tool's action type with the IDExtractor
			actionType := factory.GetToolActionType(name)
			a.idExtractor.RegisterToolActionType(name, actionType)
		}
	}

	return nil
}

// discoverMCPResources discovers available resources from the MCP server
func (a *StateAwareAgent) discoverMCPResources() error {
	a.mcpProcess.mutex.Lock()
	a.mcpProcess.reqID++
	reqID := a.mcpProcess.reqID
	a.mcpProcess.mutex.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	}

	response, err := a.sendMCPRequest(request)
	if err != nil {
		return fmt.Errorf("failed to list MCP resources: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return fmt.Errorf("MCP server error listing resources: %v", errorData)
	}

	result, exists := response["result"]
	if !exists {
		return fmt.Errorf("no result in MCP resources list response")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid result format from MCP resources list")
	}

	resources, exists := resultMap["resources"]
	if !exists {
		return fmt.Errorf("no resources field in MCP response")
	}

	resourcesArray, ok := resources.([]interface{})
	if !ok {
		return fmt.Errorf("resources field is not an array")
	}

	a.capabilityMutex.Lock()
	defer a.capabilityMutex.Unlock()

	// Clear existing resources
	a.mcpResources = make(map[string]MCPResourceInfo)

	for _, resourceInterface := range resourcesArray {
		resourceMap, ok := resourceInterface.(map[string]interface{})
		if !ok {
			continue
		}

		uri, _ := resourceMap["uri"].(string)
		name, _ := resourceMap["name"].(string)
		description, _ := resourceMap["description"].(string)
		mimeType, _ := resourceMap["mimeType"].(string)

		if uri != "" {
			a.mcpResources[uri] = MCPResourceInfo{
				URI:         uri,
				Name:        name,
				Description: description,
				MimeType:    mimeType,
			}
		}
	}

	return nil
}

// callMCPTool calls a tool via the MCP server
func (a *StateAwareAgent) callMCPTool(name string, arguments map[string]interface{}) (map[string]interface{}, error) {
	// In test mode, use the mock MCP server
	if a.testMode && a.mockMCPServer != nil {
		ctx := context.Background()
		result, err := a.mockMCPServer.CallTool(ctx, name, arguments)
		if err != nil {
			return nil, fmt.Errorf("mock MCP tool call failed: %w", err)
		}

		// Check if the result is an error response
		if result.IsError {
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
					return nil, fmt.Errorf("MCP tool returned error: %s", textContent.Text)
				}
			}

			return nil, fmt.Errorf("MCP tool returned error response")
		}

		// Convert the mcp.CallToolResult to the expected format
		if len(result.Content) > 0 {
			// Extract the first content item using robust content extraction
			var textData string
			var extractSuccess bool

			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				textData = textContent.Text
				extractSuccess = true
			} else if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				textData = textContent.Text
				extractSuccess = true
			} else if contentInterface, ok := result.Content[0].(interface{ GetText() string }); ok {
				textData = contentInterface.GetText()
				extractSuccess = true
			}

			if extractSuccess {
				var toolResult map[string]interface{}
				if err := json.Unmarshal([]byte(textData), &toolResult); err == nil {
					// Check for error in the tool result
					if errorMsg, hasError := toolResult["error"]; hasError {
						if success, hasSuccess := toolResult["success"]; hasSuccess {
							if successBool, ok := success.(bool); ok && !successBool {
								return nil, fmt.Errorf("tool execution failed: %v", errorMsg)
							}
						} else {
							// If no success field but error exists, treat as error
							return nil, fmt.Errorf("tool execution failed: %v", errorMsg)
						}
					}
					// Successfully parsed as JSON and no error
					return toolResult, nil
				}

				// If JSON parsing fails, return the text as a "text" field
				return map[string]interface{}{
					"text": textData,
				}, nil
			}
		}

		// Fallback: return the result as is
		return map[string]interface{}{
			"success": true,
		}, nil
	}

	if a.mcpProcess == nil {
		if err := a.startMCPProcess(); err != nil {
			return nil, fmt.Errorf("failed to start MCP process: %w", err)
		}
	}

	a.mcpProcess.mutex.Lock()
	a.mcpProcess.reqID++
	reqID := a.mcpProcess.reqID
	a.mcpProcess.mutex.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	response, err := a.sendMCPRequest(request)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return nil, fmt.Errorf("MCP server error: %v", errorData)
	}

	result, exists := response["result"]
	if !exists {
		return nil, fmt.Errorf("no result in MCP response")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format from MCP server")
	}

	// The MCP server returns a CallToolResult structure with Content array
	// Extract the actual tool response from the content
	if content, exists := resultMap["content"]; exists {
		if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
			if firstContent, ok := contentArray[0].(map[string]interface{}); ok {
				if text, exists := firstContent["text"]; exists {
					if textStr, ok := text.(string); ok {
						// Try to parse as JSON first
						var toolResult map[string]interface{}
						if err := json.Unmarshal([]byte(textStr), &toolResult); err == nil {
							// Check for error in the tool result
							if errorMsg, hasError := toolResult["error"]; hasError {
								if success, hasSuccess := toolResult["success"]; hasSuccess {
									if successBool, ok := success.(bool); ok && !successBool {
										return nil, fmt.Errorf("tool execution failed: %v", errorMsg)
									}
								} else {
									// If no success field but error exists, treat as error
									return nil, fmt.Errorf("tool execution failed: %v", errorMsg)
								}
							}
							// Successfully parsed as JSON and no error
							return toolResult, nil
						}

						// If JSON parsing fails, return the text as a "text" field
						return map[string]interface{}{
							"text": textStr,
						}, nil
					}
				}
			}
		}
	}

	// Fallback: return the result map as is
	return resultMap, nil
}

// ========== Infrastructure Operations ==========

// AnalyzeInfrastructureState calls the MCP server to analyze infrastructure state
func (a *StateAwareAgent) AnalyzeInfrastructureState(ctx context.Context, scanLive bool) (*types.InfrastructureState, []*types.ResourceState, []*types.ChangeDetection, error) {
	// Use direct MCP call to get the raw text response for analyze tool
	if a.mcpProcess == nil {
		if err := a.startMCPProcess(); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to start MCP process: %w", err)
		}
	}

	a.mcpProcess.mutex.Lock()
	a.mcpProcess.reqID++
	reqID := a.mcpProcess.reqID
	a.mcpProcess.mutex.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze-infrastructure-state",
			"arguments": map[string]interface{}{
				"scan_live": scanLive,
			},
		},
	}

	response, err := a.sendMCPRequest(request)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return nil, nil, nil, fmt.Errorf("MCP server error: %v", errorData)
	}

	result, exists := response["result"]
	if !exists {
		return nil, nil, nil, fmt.Errorf("no result in MCP response")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, nil, nil, fmt.Errorf("invalid result format from MCP server")
	}

	// Extract the JSON response and parse it
	var analysisText string
	var analysisData map[string]interface{}

	if content, exists := resultMap["content"]; exists {
		if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
			if firstContent, ok := contentArray[0].(map[string]interface{}); ok {
				if text, exists := firstContent["text"]; exists {
					if textStr, ok := text.(string); ok {
						analysisText = textStr

						// Try to parse as JSON to get structured data
						if err := json.Unmarshal([]byte(textStr), &analysisData); err != nil {
							a.Logger.WithError(err).Warn("Failed to parse analysis response as JSON, using text only")
						}
					}
				}
			}
		}
	}

	// Build the infrastructure state from parsed data
	currentState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
		Region:    a.awsConfig.Region,
		Version:   "1.0",
	}

	var discoveredResources []*types.ResourceState
	var driftDetections []*types.ChangeDetection

	// Extract managed resources if available
	if analysisData != nil {
		if managedRes, exists := analysisData["managed_resources"]; exists {
			if managedMap, ok := managedRes.(map[string]interface{}); ok {
				for id, resData := range managedMap {
					if resMap, ok := resData.(map[string]interface{}); ok {
						resource := &types.ResourceState{
							ID:         id,
							Type:       util.GetStringFromMap(resMap, "type"),
							Status:     util.GetStringFromMap(resMap, "status"),
							Properties: resMap,
						}
						// Set default status if not provided
						if resource.Status == "" {
							resource.Status = "managed"
						}
						currentState.Resources[id] = resource
					}
				}
			}
		}

		// Extract discovered resources if available
		if discoveredRes, exists := analysisData["discovered_resources"]; exists {
			if discoveredArray, ok := discoveredRes.([]interface{}); ok {
				for _, resData := range discoveredArray {
					if resMap, ok := resData.(map[string]interface{}); ok {
						resource := &types.ResourceState{
							ID:         util.GetStringFromMap(resMap, "id"),
							Type:       util.GetStringFromMap(resMap, "type"),
							Status:     util.GetStringFromMap(resMap, "status"),
							Properties: resMap,
						}
						// Set default status if not provided
						if resource.Status == "" {
							resource.Status = "available"
						}
						discoveredResources = append(discoveredResources, resource)
					}
				}
			}
		}

		// Extract drift detections if available
		if driftRes, exists := analysisData["drift_detections"]; exists {
			if driftArray, ok := driftRes.([]interface{}); ok {
				for _, driftData := range driftArray {
					if driftMap, ok := driftData.(map[string]interface{}); ok {
						detection := &types.ChangeDetection{
							Resource:   util.GetStringFromMap(driftMap, "resource"),
							ChangeType: util.GetStringFromMap(driftMap, "changeType"),
							Reason:     util.GetStringFromMap(driftMap, "reason"),
						}
						driftDetections = append(driftDetections, detection)
					}
				}
			}
		}
	}

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"analysis_text":    analysisText,
			"managed_count":    len(currentState.Resources),
			"discovered_count": len(discoveredResources),
			"drift_count":      len(driftDetections),
		}).Info("Infrastructure state analysis completed")
	}

	return currentState, discoveredResources, driftDetections, nil
}

// DetectInfrastructureConflicts calls the MCP server to detect conflicts
func (a *StateAwareAgent) DetectInfrastructureConflicts(ctx context.Context, autoResolve bool) ([]*types.ConflictResolution, error) {
	result, err := a.callMCPTool("detect-infrastructure-conflicts", map[string]interface{}{
		"auto_resolve": autoResolve,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to detect infrastructure conflicts: %w", err)
	}

	// Parse the response - it might be text or JSON
	var conflicts []*types.ConflictResolution

	// Log the response for debugging
	a.Logger.WithField("response", result).Info("Conflict detection completed")

	return conflicts, nil
}

// PlanInfrastructureDeployment calls the MCP server to plan deployment
func (a *StateAwareAgent) PlanInfrastructureDeployment(ctx context.Context, targetResources []string, includeLevels bool) ([]string, [][]string, error) {
	arguments := map[string]interface{}{
		"include_levels": includeLevels,
	}

	if len(targetResources) > 0 {
		arguments["target_resources"] = targetResources
	}

	result, err := a.callMCPTool("plan-infrastructure-deployment", arguments)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to plan infrastructure deployment: %w", err)
	}

	// Parse the response - it might be text or JSON
	var deploymentOrder []string
	var deploymentLevels [][]string

	// Log the response for debugging
	a.Logger.WithField("response", result).Info("Deployment planning completed")

	return deploymentOrder, deploymentLevels, nil
}

// VisualizeDependencyGraph calls the MCP server to visualize dependency graph
func (a *StateAwareAgent) VisualizeDependencyGraph(ctx context.Context, format string, includeBottlenecks bool) (string, []string, error) {
	result, err := a.callMCPTool("visualize-dependency-graph", map[string]interface{}{
		"format":              format,
		"include_bottlenecks": includeBottlenecks,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to visualize dependency graph: %w", err)
	}

	// Extract visualization from response - it comes as text from MCP tool
	visualization := ""
	if text, exists := result["text"]; exists {
		if textStr, ok := text.(string); ok {
			visualization = textStr
		}
	} else if vis, exists := result["visualization"]; exists {
		// Fallback: check for "visualization" field
		if visStr, ok := vis.(string); ok {
			visualization = visStr
		}
	}

	// Parse bottlenecks if included (for now, return empty since they're part of the text)
	var bottlenecks []string

	a.Logger.WithField("response", result).Info("Dependency graph visualization completed")

	return visualization, bottlenecks, nil
}

// ExportInfrastructureState calls the MCP server to export infrastructure state
func (a *StateAwareAgent) ExportInfrastructureState(ctx context.Context, includeDiscovered bool) (string, error) {
	return a.ExportInfrastructureStateWithOptions(ctx, includeDiscovered, true)
}

// ExportInfrastructureStateWithOptions calls the MCP server to export infrastructure state with full control
func (a *StateAwareAgent) ExportInfrastructureStateWithOptions(ctx context.Context, includeDiscovered, includeManaged bool) (string, error) {
	// Use a direct MCP call to get the raw text response
	if a.mcpProcess == nil {
		if err := a.startMCPProcess(); err != nil {
			return "", fmt.Errorf("failed to start MCP process: %w", err)
		}
	}

	a.mcpProcess.mutex.Lock()
	a.mcpProcess.reqID++
	reqID := a.mcpProcess.reqID
	a.mcpProcess.mutex.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "export-infrastructure-state",
			"arguments": map[string]interface{}{
				"include_managed": includeManaged,
			},
		},
	}

	response, err := a.sendMCPRequest(request)
	if err != nil {
		return "", fmt.Errorf("MCP tool call failed: %w", err)
	}

	if errorData, exists := response["error"]; exists {
		return "", fmt.Errorf("MCP server error: %v", errorData)
	}

	result, exists := response["result"]
	if !exists {
		return "", fmt.Errorf("no result in MCP response")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid result format from MCP server")
	}

	// Extract the JSON text directly from the MCP tool response
	if content, exists := resultMap["content"]; exists {
		if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
			if firstContent, ok := contentArray[0].(map[string]interface{}); ok {
				if text, exists := firstContent["text"]; exists {
					if textStr, ok := text.(string); ok {
						a.Logger.WithField("state_length", len(textStr)).Info("Infrastructure state export completed")
						return textStr, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("failed to extract state JSON from MCP response")
}

// AddResourceToState calls the MCP server to add a resource to state
func (a *StateAwareAgent) AddResourceToState(resourceState *types.ResourceState) error {
	a.Logger.WithField("resource_id", resourceState.ID).Info("Adding resource to state via MCP server")

	// Call the MCP tool to add the resource to state
	result, err := a.callMCPTool("add-resource-to-state", map[string]interface{}{
		"resource_id":   resourceState.ID,
		"resource_name": resourceState.Name,
		"description":   resourceState.Description,
		"resource_type": resourceState.Type,
		"status":        resourceState.Status,
		"properties":    resourceState.Properties,
		"dependencies":  resourceState.Dependencies,
	})
	if err != nil {
		return fmt.Errorf("failed to add resource to state via MCP: %w", err)
	}

	a.Logger.WithField("result", result).Debug("Resource added to state via MCP server")
	return nil
}
