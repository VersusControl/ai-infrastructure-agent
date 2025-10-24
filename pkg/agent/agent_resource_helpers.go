package agent

import (
	"fmt"
	"time"
)

// ========== Interface defines ==========

// ResourceHelpersInterface defines resource management and state handling functionality
//
// Available Functions:
//   - waitForResourceReady()        : Wait for AWS resources to be in ready state
//   - checkResourceState()          : Check if a resource is ready
//   - checkNATGatewayState()        : Check NAT Gateway specific state
//   - checkRDSInstanceState()       : Check RDS instance specific state
//   - checkEC2InstanceState()       : Check EC2 instance specific state (stopped/running)
//
// This file provides helper functions for resource readiness checks and state verification
// during infrastructure operations. Different resource types have specialized state checking.
//
// Supported resource wait operations:
//   - create-nat-gateway: Wait for NAT gateway to be "available" (5 min timeout)
//   - create-rds-db-instance: Wait for RDS instance to be "available" (15 min timeout)
//   - stop-ec2-instance: Wait for EC2 instance to be "stopped" (3 min timeout)
//   - start-ec2-instance: Wait for EC2 instance to be "running" (3 min timeout)
//   - modify-ec2-instance-type: Wait for modification to complete, instance remains "stopped" (2 min timeout)
//
// Usage Example:
//   1. err := agent.waitForResourceReady("stop-ec2-instance", instanceID)
//   2. err := agent.waitForResourceReady("modify-ec2-instance-type", instanceID)
//   3. err := agent.waitForResourceReady("start-ec2-instance", instanceID)
//   4. ready, err := agent.checkEC2InstanceState(instanceID, "stopped")

// ========== Resource Management Helper Functions ==========

// waitForResourceReady waits for AWS resources to be in a ready state before continuing
func (a *StateAwareAgent) waitForResourceReady(toolName, resourceID string) error {
	if a.testMode {
		return nil
	}

	// Determine if this resource type needs waiting
	needsWaiting := false
	maxWaitTime := 5 * time.Minute
	checkInterval := 15 * time.Second

	switch toolName {
	case "create-nat-gateway":
		needsWaiting = true
	case "create-rds-db-instance", "create-database":
		needsWaiting = true
		maxWaitTime = 15 * time.Minute // RDS instances can take longer
	case "stop-ec2-instance":
		needsWaiting = true
	case "start-ec2-instance":
		needsWaiting = true
	case "modify-ec2-instance-type":
		needsWaiting = true
	case "create-internet-gateway", "create-vpc", "create-subnet":
		// These are typically available immediately
		needsWaiting = false
	default:
		// For other resources, don't wait
		needsWaiting = false
	}

	if !needsWaiting {
		return nil
	}

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":      toolName,
		"resource_id":    resourceID,
		"max_wait_time":  maxWaitTime,
		"check_interval": checkInterval,
	}).Info("Waiting for resource to be ready")

	startTime := time.Now()
	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			elapsed := time.Since(startTime)
			return fmt.Errorf("timeout waiting for %s %s to be ready after %v", toolName, resourceID, elapsed)

		case <-ticker.C:
			ready, err := a.checkResourceState(toolName, resourceID)
			if err != nil {
				a.Logger.WithError(err).WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
				}).Warn("Error checking resource state, will retry")
				continue
			}

			if ready {
				elapsed := time.Since(startTime)
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
					"elapsed":     elapsed,
				}).Info("Resource is ready")
				return nil
			}
		}
	}
}

// checkResourceState checks if a specific AWS resource is in a ready state
func (a *StateAwareAgent) checkResourceState(toolName, resourceID string) (bool, error) {
	switch toolName {
	case "create-nat-gateway":
		return a.checkNATGatewayState(resourceID)
	case "create-rds-db-instance", "create-database":
		return a.checkRDSInstanceState(resourceID)
	case "stop-ec2-instance":
		return a.checkEC2InstanceState(resourceID, "stopped")
	case "start-ec2-instance":
		return a.checkEC2InstanceState(resourceID, "running")
	case "modify-ec2-instance-type":
		// After modification, check if instance is still stopped (ready for start)
		return a.checkEC2InstanceState(resourceID, "stopped")
	default:
		// For unknown resource types, assume they're ready
		return true, nil
	}
}

// checkNATGatewayState checks if a NAT gateway is available
func (a *StateAwareAgent) checkNATGatewayState(natGatewayID string) (bool, error) {
	// Try to use MCP tool to describe the NAT gateway if available
	result, err := a.callMCPTool("describe-nat-gateways", map[string]interface{}{
		"natGatewayIds": []string{natGatewayID},
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"nat_gateway_id": natGatewayID,
			"error":          err.Error(),
		}).Warn("describe-nat-gateways tool not available, using time-based wait")

		// NAT gateways typically take 2-3 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(30 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if natGateways, ok := result["natGateways"].([]interface{}); ok && len(natGateways) > 0 {
		if natGateway, ok := natGateways[0].(map[string]interface{}); ok {
			if state, ok := natGateway["state"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"nat_gateway_id": natGatewayID,
					"state":          state,
				}).Debug("NAT gateway state check")

				return state == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine NAT gateway state from response")
}

// checkRDSInstanceState checks if an RDS instance is available
func (a *StateAwareAgent) checkRDSInstanceState(dbInstanceID string) (bool, error) {
	// Try to use MCP tool to describe the RDS instance if available
	result, err := a.callMCPTool("describe-db-instances", map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceID,
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"db_instance_id": dbInstanceID,
			"error":          err.Error(),
		}).Warn("describe-db-instances tool not available, using time-based wait")

		// RDS instances typically take 5-10 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(60 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if dbInstances, ok := result["dbInstances"].([]interface{}); ok && len(dbInstances) > 0 {
		if dbInstance, ok := dbInstances[0].(map[string]interface{}); ok {
			if status, ok := dbInstance["dbInstanceStatus"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"db_instance_id": dbInstanceID,
					"status":         status,
				}).Debug("RDS instance state check")

				return status == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine RDS instance state from response")
}

// checkEC2InstanceState checks if an EC2 instance is in the expected state
func (a *StateAwareAgent) checkEC2InstanceState(instanceID, expectedState string) (bool, error) {
	// Try to use get-ec2-instance tool for efficient single instance lookup
	result, err := a.callMCPTool("get-ec2-instance", map[string]interface{}{
		"instanceId": instanceID,
	})
	if err != nil {
		a.Logger.WithFields(map[string]interface{}{
			"instance_id": instanceID,
			"error":       err.Error(),
		}).Warn("get-ec2-instance tool not available for state check")

		// If we can't check the state, wait a bit and assume success
		// This is not ideal but prevents blocking the workflow
		time.Sleep(15 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	// Response format: {"instanceId": "i-xxx", "state": "stopped", ...}
	var currentState string
	if state, exists := result["state"]; exists {
		currentState = fmt.Sprintf("%v", state)
	} else if state, exists := result["State"]; exists {
		// Also check PascalCase for compatibility
		currentState = fmt.Sprintf("%v", state)
	}

	if currentState == "" {
		return false, fmt.Errorf("could not determine state for instance %s from response", instanceID)
	}

	return currentState == expectedState, nil
}
