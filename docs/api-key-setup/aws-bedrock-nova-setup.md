# AWS Bedrock Nova Setup Guide

This guide will help you configure AWS Bedrock with Amazon Nova models for use with the AI Infrastructure Agent.

## Prerequisites

- AWS Account with appropriate permissions
- AWS CLI installed and configured
- Understanding of AWS IAM permissions

## Overview

Amazon Nova models are available through AWS Bedrock and offer:
- **Regional deployment**: Models are deployed in specific AWS regions
- **No API key required**: Uses your existing AWS credentials
- **Enterprise security**: Built on AWS security infrastructure
- **Cost-effective pricing**: Pay only for what you use

## Step-by-Step Setup

### 1. Check Regional Availability

Amazon Nova models are available in select AWS regions. Check current availability:

| Region | Region Code | Nova Models Available |
|--------|-------------|----------------------|
| US East (N. Virginia) | `us-east-1` | ✅ Nova Micro, Lite, Pro |
| US West (Oregon) | `us-west-2` | ✅ Nova Micro, Lite, Pro |
| Europe (Ireland) | `eu-west-1` | ✅ Nova Micro, Lite, Pro |
| Asia Pacific (Singapore) | `ap-southeast-1` | ✅ Nova Micro, Lite, Pro |

⚠️ **Important**: Model availability changes frequently. Check the [AWS Bedrock Console](https://console.aws.amazon.com/bedrock/) for the most current information.

### 2. Configure AWS Credentials

Choose one of the following methods:

#### Option A: AWS CLI Configuration
```bash
# Install AWS CLI if not already installed
# Visit: https://aws.amazon.com/cli/

# Configure credentials
aws configure
```

You'll need to provide:
- **AWS Access Key ID**: Your IAM user access key
- **AWS Secret Access Key**: Your IAM user secret key
- **Default region**: Choose a region with Nova availability (e.g., `us-east-1`)
- **Default output format**: `json` (recommended)

#### Option B: Environment Variables
```bash
export AWS_ACCESS_KEY_ID="your-access-key-id"
export AWS_SECRET_ACCESS_KEY="your-secret-access-key"
export AWS_DEFAULT_REGION="us-east-1"
```

#### Option C: IAM Roles (Recommended for EC2/ECS)
If running on AWS infrastructure, use IAM roles instead of access keys.

### 3. Set Up IAM Permissions

Create an IAM policy with the required Bedrock permissions:

#### Minimum Required Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/amazon.nova-micro-v1:0",
        "arn:aws:bedrock:*::foundation-model/amazon.nova-lite-v1:0",
        "arn:aws:bedrock:*::foundation-model/amazon.nova-pro-v1:0"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:ListFoundationModels",
        "bedrock:GetFoundationModel"
      ],
      "Resource": "*"
    }
  ]
}
```

#### Create IAM Policy and User

1. **Create the policy**:
   ```bash
   # Save the JSON policy above as bedrock-nova-policy.json
   aws iam create-policy \
     --policy-name BedrockNovaAccess \
     --policy-document file://bedrock-nova-policy.json
   ```

2. **Create IAM user** (if not using existing user):
   ```bash
   aws iam create-user --user-name ai-infrastructure-agent
   ```

3. **Attach policy to user**:
   ```bash
   aws iam attach-user-policy \
     --user-name ai-infrastructure-agent \
     --policy-arn arn:aws:iam::YOUR-ACCOUNT-ID:policy/BedrockNovaAccess
   ```

4. **Create access keys**:
   ```bash
   aws iam create-access-key --user-name ai-infrastructure-agent
   ```

### 4. Enable Nova Models in AWS Console

⚠️ **Critical Step**: You must explicitly request access to Nova models through the AWS Console.

1. **Navigate to AWS Bedrock Console**:
   - Go to [AWS Bedrock Console](https://console.aws.amazon.com/bedrock/)
   - Select your preferred region (e.g., us-east-1)

2. **Request Model Access**:
   - Click **"Model access"** in the left sidebar
   - Find **Amazon Nova** models in the list
   - Click **"Request model access"** for each Nova model you want to use:
     - `amazon.nova-micro-v1:0` (Fastest, most cost-effective)
     - `amazon.nova-lite-v1:0` (Balanced performance)
     - `amazon.nova-pro-v1:0` (Most capable)

3. **Wait for Approval**:
   - Model access is usually granted within minutes
   - Status will change from "Not requested" → "In progress" → "Access granted"
   - You'll receive email notifications about approval status

4. **Verify Access**:
   ```bash
   # List available models
   aws bedrock list-foundation-models --region us-east-1
   
   # Check for Nova models in the output
   aws bedrock list-foundation-models --region us-east-1 | grep -i nova
   ```

### 5. Configure the Agent

Update your `config.yaml`:

```yaml
agent:
  provider: "bedrock"              # or "nova"
  model: "amazon.nova-micro-v1:0"  # Choose your model
  max_tokens: 4000
  temperature: 0.1
  dry_run: true
  region: "us-east-1"              # Must match your Nova region
```

### 6. Available Nova Models

Choose the appropriate model based on your needs:

| Model | Model ID | Best For | Speed | Cost |
|-------|----------|----------|-------|------|
| **Nova Micro** | `amazon.nova-micro-v1:0` | Simple tasks, high throughput | Fastest | Lowest |
| **Nova Lite** | `amazon.nova-lite-v1:0` | Balanced performance | Fast | Medium |
| **Nova Pro** | `amazon.nova-pro-v1:0` | Complex reasoning, planning | Medium | Higher |

## Regional Considerations

### Important Notes for Multi-Region Usage

- **Consistent model availability**: All Nova models (Micro, Lite, Pro) are now available across major regions
- **Latency**: Choose regions close to your location for better performance
- **Costs**: Pricing may vary slightly between regions
- **Compliance**: Consider data residency requirements

### Testing Regional Access

```bash
# Test model access in different regions
aws bedrock list-foundation-models --region us-east-1 | grep nova
aws bedrock list-foundation-models --region us-west-2 | grep nova
aws bedrock list-foundation-models --region eu-west-1 | grep nova
aws bedrock list-foundation-models --region ap-southeast-1 | grep nova
```

## Testing Your Setup

### 1. Test AWS Credentials

```bash
# Verify AWS credentials
aws sts get-caller-identity

# Should return your account ID, user ARN, etc.
```

### 2. Test Bedrock Access

```bash
# List all foundation models
aws bedrock list-foundation-models --region us-east-1

# Test Nova model specifically
aws bedrock invoke-model \
  --region us-east-1 \
  --model-id amazon.nova-micro-v1:0 \
  --body '{"messages":[{"role":"user","content":[{"text":"Hello, respond with: Bedrock Nova working correctly"}]}],"max_tokens":100,"temperature":0.1}' \
  --cli-binary-format raw-in-base64-out \
  /tmp/bedrock-response.json

# View the response
cat /tmp/bedrock-response.json
```

### 3. Test with AI Infrastructure Agent

Start the agent and verify connectivity:

```bash
# Set environment
export AWS_DEFAULT_REGION="us-east-1"

# Start the web UI
./scripts/run-web-ui.sh

# Check logs for successful Nova initialization
tail -f logs/app.log
```

## Troubleshooting

### Common Issues

#### "AccessDeniedException: User is not authorized to perform: bedrock:InvokeModel"

**Solutions**:
1. Verify IAM permissions are correctly attached
2. Check the resource ARNs in your policy
3. Ensure you're using the correct AWS region
4. Wait a few minutes for IAM changes to propagate

```bash
# Debug IAM permissions
aws sts get-caller-identity
aws iam list-attached-user-policies --user-name your-username
```

#### "ValidationException: The model 'amazon.nova-micro-v1:0' is not supported in region 'us-west-2'"

**Solutions**:
1. Check model availability in your region
2. Switch to a supported region (e.g., us-east-1)
3. Update your config.yaml with the correct region

```bash
# Check model availability by region
aws bedrock list-foundation-models --region us-east-1 --query 'modelSummaries[?contains(modelId, `nova`)]'
```

#### "ValidationException: You don't have access to the model with the specified model ID"

**Solutions**:
1. Request model access through AWS Bedrock Console
2. Wait for access approval (usually takes a few minutes)
3. Verify approval status in the console

#### "CredentialsError: Unable to locate credentials"

**Solutions**:
```bash
# Check AWS credential configuration
aws configure list

# Set credentials if missing
aws configure

# Or use environment variables
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export AWS_DEFAULT_REGION="us-east-1"
```

### Performance Optimization

#### Choose the Right Model

- **Development/Testing**: Use `nova-micro-v1:0` for cost efficiency
- **Production**: Use `nova-lite-v1:0` or `nova-pro-v1:0` based on complexity
- **Complex Infrastructure**: Use `nova-pro-v1:0` for detailed planning

#### Optimize Configuration

```yaml
agent:
  provider: "bedrock"
  model: "amazon.nova-micro-v1:0"
  max_tokens: 6000                    # Increase for complex tasks
  temperature: 0.1                    # Lower for consistent outputs
  region: "us-east-1"                 # Choose closest region
  enable_debug: false                 # Disable in production
```

## Cost Management

### Pricing Structure

Nova models use token-based pricing:
- **Input tokens**: Charged per 1K tokens
- **Output tokens**: Charged per 1K tokens  
- **No minimum charges**: Pay only for usage

### Cost Optimization Tips

1. **Start with Nova Micro** for development
2. **Set reasonable token limits** (4000-6000 for infrastructure tasks)
3. **Use dry-run mode** during development
4. **Monitor usage** through AWS Billing Dashboard
5. **Set up billing alerts** to avoid surprises

### Monitor Costs

```bash
# Check current month's Bedrock usage
aws ce get-cost-and-usage \
  --time-period Start=2024-01-01,End=2024-01-31 \
  --granularity MONTHLY \
  --metrics BlendedCost \
  --group-by Type=DIMENSION,Key=SERVICE \
  --query 'ResultsByTime[0].Groups[?Keys[0]==`Amazon Bedrock`]'
```

## Security Best Practices

### IAM Security

- ✅ **Use least privilege**: Only grant required permissions
- ✅ **Use IAM roles** when possible (instead of access keys)
- ✅ **Rotate credentials regularly**
- ✅ **Enable CloudTrail** for API logging
- ✅ **Use separate credentials** for different environments

### Network Security

- ✅ **Use VPC endpoints** for private connectivity
- ✅ **Implement proper security groups**
- ✅ **Enable encryption in transit and at rest**

## Support and Resources

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [Amazon Nova Model Documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/nova-models.html)
- [AWS Bedrock Pricing](https://aws.amazon.com/bedrock/pricing/)
- [AWS Support](https://aws.amazon.com/support/)

---

**Next Steps**: After setting up AWS Bedrock Nova, return to the main [README](../README.md) to start using the AI Infrastructure Agent.