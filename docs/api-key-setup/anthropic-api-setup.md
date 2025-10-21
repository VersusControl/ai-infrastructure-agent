# Anthropic Claude API Key Setup Guide

This guide will help you obtain an Anthropic API key for use with the AI Infrastructure Agent.

## Prerequisites

- Anthropic account
- Valid payment method (Anthropic requires billing setup for API access)

## Step-by-Step Instructions

### 1. Create an Anthropic Account

1. Visit [console.anthropic.com](https://console.anthropic.com/)
2. Click **"Sign up"** if you don't have an account
3. Complete the registration process with your email
4. Verify your email address

### 2. Set Up Billing

⚠️ **Important**: Anthropic requires a valid payment method to use the API.

1. Once logged in, navigate to [Billing Settings](https://console.anthropic.com/settings/billing)
2. Click **"Add payment method"**
3. Enter your credit card information
4. Set up usage limits (recommended):
   - **Monthly budget**: Set a reasonable monthly limit (e.g., $10-50)
   - **Rate limits**: Configure to prevent unexpected usage spikes

### 3. Generate API Key

1. Navigate to [API Keys](https://console.anthropic.com/settings/keys)
2. Click **"Create Key"**
3. Give your key a descriptive name (e.g., "AI Infrastructure Agent")
4. **Copy the key immediately** - you won't be able to see it again
5. Store the key securely (consider using a password manager)

### 4. Choose Your Claude Model

Claude 4 models for infrastructure tasks:

| Model | Best For | Cost | Context Window |
|-------|----------|------|----------------|
| `claude-sonnet-4-5-20250929` | **Recommended** - Best balance for agents and coding | $3 / $15 per MTok | 200K tokens (1M beta) |
| `claude-haiku-4-5-20251001` | Fastest with near-frontier intelligence | $1 / $5 per MTok | 200K tokens |
| `claude-opus-4-1-20250805` | Exceptional for specialized reasoning tasks | $15 / $75 per MTok | 200K tokens |

**Model Aliases** (auto-update to latest versions):
- `claude-sonnet-4-5` → `claude-sonnet-4-5-20250929`
- `claude-haiku-4-5` → `claude-haiku-4-5-20251001`
- `claude-opus-4-1` → `claude-opus-4-1-20250805`

**Key Features:**
- **Max Output**: 64K tokens (Sonnet/Haiku), 32K tokens (Opus)
- **Extended Thinking**: All models support extended reasoning
- **Priority Tier**: Available for all models
- **Knowledge Cutoff**: January 2025 (Sonnet/Opus), February 2025 (Haiku)
- **Vision**: All models support text and image input

**Recommended**: Start with `claude-sonnet-4-5-20250929` - it offers the best balance of intelligence, speed, and cost for infrastructure tasks, with exceptional performance in coding and agentic workflows.

> **Note**: Use specific model versions (with dates) in production for consistent behavior. Aliases automatically point to the newest version and may change when new snapshots are released.

### 5. Configure the Agent

Set your API key as an environment variable:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export ANTHROPIC_API_KEY="sk-ant-your-api-key-here"

# Or set it temporarily for the current session
export ANTHROPIC_API_KEY="sk-ant-your-api-key-here"
```

Update your `config.yaml`:

```yaml
agent:
  provider: "anthropic"
  model: "claude-sonnet-4-5-20250929"    # Recommended
  max_tokens: 4000
  temperature: 0.1
```

Or use the example configuration:

```bash
# Copy the Anthropic example config
cp config.anthropic.yaml.example config.yaml

# Set your API key
export ANTHROPIC_API_KEY="sk-ant-your-api-key-here"
```

## Usage Monitoring

### Track Your Usage

1. Visit [Usage Dashboard](https://console.anthropic.com/settings/usage)
2. Monitor your monthly spending
3. View detailed usage statistics

### Cost Optimization Tips

- **Start with `claude-sonnet-4-5-20250929`** for best value and performance
- **Use `claude-haiku-4-5-20251001`** for simple, frequent operations
- **Use dry-run mode** to test without making API calls
- **Set conservative token limits** in config (start with 2000-4000)
- **Monitor usage regularly** especially during development
- **Use specific model versions** (not aliases) in production for consistent costs

### Current Pricing (USD per 1M tokens)

| Model | Input Tokens | Output Tokens | Max Output | Notes |
|-------|-------------|---------------|------------|--------|
| `claude-sonnet-4-5-20250929` | $3.00 | $15.00 | 64K tokens | **Best value** - Recommended |
| `claude-haiku-4-5-20251001` | $1.00 | $5.00 | 64K tokens | Fastest, cost-effective |
| `claude-opus-4-1-20250805` | $15.00 | $75.00 | 32K tokens | Specialized reasoning |

**Additional Pricing:**
- **Long Context** (>200K tokens): Higher rates apply for Sonnet 4.5's 1M token context window
- **Extended Thinking**: Additional costs for reasoning-intensive tasks
- **Batch API**: Discounted rates available for batch processing
- **Prompt Caching**: Reduced costs for repeated context

💡 **Tip**: Infrastructure tasks typically use 2000-6000 tokens per request. `claude-sonnet-4-5-20250929` costs ~$0.006-0.018 per request for input and ~$0.03-0.09 for output.

## Verify Your Setup

Test your API key with a simple curl command:

```bash
# Check API key is set
echo $ANTHROPIC_API_KEY

# Test API connectivity
curl https://api.anthropic.com/v1/messages \
  -H "content-type: application/json" \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, Claude!"}
    ]
  }'
```

If successful, you'll receive a JSON response with Claude's greeting.

## Start the Agent

```bash
# Start the agent with Anthropic Claude
./bin/web-ui

# Or with explicit config
./bin/web-ui -config config.anthropic.yaml
```

## Troubleshooting

### Common Issues

1. **"Authentication error"**
   - Verify your API key is correct
   - Check that the key is properly set in environment variable
   - Ensure there are no extra spaces or quotes

2. **"Rate limit exceeded"**
   - You've hit your usage limit
   - Wait for the rate limit to reset (usually per minute)
   - Consider upgrading your plan or spreading out requests

3. **"Billing required"**
   - Add a payment method in the [Billing Settings](https://console.anthropic.com/settings/billing)
   - Verify your payment method is active

4. **"Model not found"**
   - Check that you're using a valid model name
   - Ensure model is available in your region
   - Verify your account has access to the model

### Getting Help

- **Anthropic Documentation**: [docs.anthropic.com](https://docs.anthropic.com/)
- **API Reference**: [docs.anthropic.com/claude/reference](https://docs.anthropic.com/claude/reference)
- **Support**: [support.anthropic.com](https://support.anthropic.com/)
- **Status Page**: [status.anthropic.com](https://status.anthropic.com/)

## Security Best Practices

- **Never commit API keys** to version control
- **Use environment variables** for sensitive data
- **Rotate keys regularly** (every 90 days recommended)
- **Use separate keys** for development and production
- **Monitor usage** for unexpected patterns
- **Set usage limits** to prevent unexpected charges

## Next Steps

- [Getting Started Guide](/getting-started.md)
- [Architecture Overview](/architecture/architecture-overview.md)
- [Working with EC2 Instances](/examples/working-with-ec2-instance.md)
- [Working with VPC](/examples/working-with-vpc.md)
