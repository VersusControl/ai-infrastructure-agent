package main

import (
	"fmt"
	"os"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
)

func main() {
	logger := logging.NewLogger("info", "text")

	// Try to create a client with a dummy region
	// We don't need actual connectivity for this test, just the struct
	client, err := aws.NewClient("us-east-1", logger)
	if err != nil {
		// If it fails (e.g. no creds), we might not be able to proceed easily with the factory
		// as it expects a *aws.Client.
		// However, for the purpose of checking the factory registration logic,
		// we can try to proceed if we can mock it, but here we just report the error.
		// Note: AWS SDK v2 LoadDefaultConfig usually succeeds even without creds,
		// it just returns a config with empty credentials provider chain if none found.
		fmt.Printf("Warning: Failed to create AWS client: %v\n", err)
	}

	factory := tools.NewToolFactory(client, logger)
	supportedTools := factory.GetSupportedToolTypes()

	eksTools := []string{
		"create-eks-cluster",
		"delete-eks-cluster",
		"get-eks-cluster",
		"list-eks-clusters",
		"create-eks-node-group",
		"list-eks-node-groups",
	}

	fmt.Println("Verifying EKS Tools Registration...")
	fmt.Println("----------------------------------")

	missing := []string{}
	foundCount := 0

	// Check if tools are in the supported map
	for _, tool := range eksTools {
		found := false
		for _, toolsList := range supportedTools {
			for _, t := range toolsList {
				if t == tool {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if found {
			fmt.Printf("✅ %s is registered\n", tool)
			foundCount++
		} else {
			fmt.Printf("❌ %s is MISSING\n", tool)
			missing = append(missing, tool)
		}
	}

	fmt.Println("----------------------------------")
	if len(missing) == 0 {
		fmt.Println("SUCCESS: All EKS tools are correctly registered!")
		os.Exit(0)
	} else {
		fmt.Printf("FAILURE: %d tools are missing.\n", len(missing))
		os.Exit(1)
	}
}
