package agent

// // ========== Interface defines ==========
// // UtilityHelpersInterface defines utility helper functionality
// //
// // Available Functions:
// //   - countRules()                  : Count number of rules in security group rule sets
// //   - extractOpenPorts()            : Extract all open ports from security group rules
// //   - addPortsFromRule()            : Add ports from a single rule to port set
// //   - hasPortInRules()              : Check if security group rules include specific port
// //   - ruleIncludesPort()            : Check if single rule includes target port
// //
// // This file provides utility functions for security group analysis, port
// // management, and other common infrastructure analysis operations.
// //
// // Usage Example:
// //   1. portCount := agent.countRules(securityGroupRules)
// //   2. openPorts := agent.extractOpenPorts(ingressRules)
// //   3. hasHTTP := agent.hasPortInRules(rules, 80)

// // ========== Utility Helper Functions ==========

// // countRules counts the number of rules in a rule set
// func (a *StateAwareAgent) countRules(rulesInterface interface{}) int {
// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		return len(rules)
// 	}
// 	if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		return len(rules)
// 	}
// 	return 0
// }

// // extractOpenPorts extracts all open ports from security group rules
// func (a *StateAwareAgent) extractOpenPorts(rulesInterface interface{}) []int {
// 	var ports []int
// 	portSet := make(map[int]bool)

// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		for _, rule := range rules {
// 			if ruleMap, ok := rule.(map[string]interface{}); ok {
// 				a.addPortsFromRule(ruleMap, portSet)
// 			}
// 		}
// 	} else if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		for _, rule := range rules {
// 			a.addPortsFromRule(rule, portSet)
// 		}
// 	}

// 	for port := range portSet {
// 		ports = append(ports, port)
// 	}

// 	return ports
// }

// // addPortsFromRule adds ports from a single rule to the port set
// func (a *StateAwareAgent) addPortsFromRule(rule map[string]interface{}, portSet map[int]bool) {
// 	if fromPort, exists := rule["from_port"]; exists {
// 		if toPort, exists := rule["to_port"]; exists {
// 			if from, ok := fromPort.(int); ok {
// 				if to, ok := toPort.(int); ok {
// 					for port := from; port <= to; port++ {
// 						portSet[port] = true
// 					}
// 				}
// 			}
// 		}
// 	}
// }

// // hasPortInRules checks if security group rules include a specific port
// func (a *StateAwareAgent) hasPortInRules(rulesInterface interface{}, targetPort int) bool {
// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		for _, rule := range rules {
// 			if ruleMap, ok := rule.(map[string]interface{}); ok {
// 				if a.ruleIncludesPort(ruleMap, targetPort) {
// 					return true
// 				}
// 			}
// 		}
// 	} else if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		for _, rule := range rules {
// 			if a.ruleIncludesPort(rule, targetPort) {
// 				return true
// 			}
// 		}
// 	}
// 	return false
// }

// // ruleIncludesPort checks if a single rule includes the target port
// func (a *StateAwareAgent) ruleIncludesPort(rule map[string]interface{}, targetPort int) bool {
// 	if fromPort, exists := rule["from_port"]; exists {
// 		if toPort, exists := rule["to_port"]; exists {
// 			if from, ok := fromPort.(int); ok {
// 				if to, ok := toPort.(int); ok {
// 					return from <= targetPort && targetPort <= to
// 				}
// 			}
// 		}
// 	}
// 	return false
// }
