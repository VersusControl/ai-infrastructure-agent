# OpenAI API Key Setup Guide

This guide will help you obtain an OpenAI API key for use with the AI Infrastructure Agent.

## Prerequisites

- OpenAI account (personal or organization)
- Valid payment method (OpenAI requires billing setup for API access)

## Step-by-Step Instructions

### 1. Create an OpenAI Account

1. Visit [platform.openai.com](https://platform.openai.com/)
2. Click **"Sign up"** if you don't have an account
3. Complete the registration process with your email and phone number
4. Verify your email address

### 2. Set Up Billing

⚠️ **Important**: OpenAI requires a valid payment method to use the API, even for free tier usage.

1. Once logged in, navigate to [Billing Settings](https://platform.openai.com/account/billing)
2. Click **"Add payment method"**
3. Enter your credit card information
4. Set up usage limits (recommended):
   - **Soft limit**: Set a reasonable monthly limit (e.g., $10-50)
   - **Hard limit**: Set a maximum limit to prevent unexpected charges

### 3. Generate API Key

1. Navigate to [API Keys](https://platform.openai.com/account/api-keys)
2. Click **"Create new secret key"**
3. Give your key a descriptive name (e.g., "AI Infrastructure Agent")
4. **Copy the key immediately** - you won't be able to see it again
5. Store the key securely (consider using a password manager)

### 4. Choose Your Model

Common models for infrastructure tasks:

| Model | Best For | Cost | Context Window |
|-------|----------|------|----------------|
| `gpt-4o` | Complex infrastructure planning, most capable | Higher | 128K tokens |
| `gpt-4o-mini` | Simple tasks, cost-effective, recommended for most users | Lower | 128K tokens |
| `o1-preview` | Complex reasoning, advanced problem solving | Highest | 128K tokens |
| `o1-mini` | Fast reasoning tasks, cost-effective | Medium | 128K tokens |
| `gpt-4-turbo` | Previous generation, still capable | Medium | 128K tokens |
| `gpt-3.5-turbo` | Legacy model, basic operations only | Lowest | 16K tokens |

**Recommended**: Start with `gpt-4o-mini` for development and testing, upgrade to `gpt-4o` for production use.

### 5. Configure the Agent

Set your API key as an environment variable:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export OPENAI_API_KEY="sk-your-api-key-here"

# Or set it temporarily for the current session
export OPENAI_API_KEY="sk-your-api-key-here"
```

Update your `config.yaml`:

```yaml
agent:
  provider: "openai"
  model: "gpt-4o-mini"    # Choose based on your needs
  max_tokens: 4000
  temperature: 0.1
```

## Usage Monitoring

### Track Your Usage

1. Visit [Usage Dashboard](https://platform.openai.com/account/usage)
2. Monitor your daily/monthly spending
3. Set up usage alerts

### Cost Optimization Tips

- **Start with `gpt-4o-mini`** for testing and simple tasks
- **Use dry-run mode** to test without making API calls
- **Set conservative token limits** in config (start with 2000-4000)
- **Monitor usage regularly** especially during development

### Current Pricing (USD per 1M tokens)

| Model | Input Tokens | Output Tokens | Notes |
|-------|-------------|---------------|--------|
| `gpt-4o` | $2.50 | $10.00 | Most capable |
| `gpt-4o-mini` | $0.15 | $0.60 | Best value for most tasks |
| `o1-preview` | $15.00 | $60.00 | Advanced reasoning |
| `o1-mini` | $3.00 | $12.00 | Fast reasoning |
| `gpt-4-turbo` | $10.00 | $30.00 | Previous generation |

💡 **Tip**: Infrastructure tasks typically use 2000-6000 tokens per request. `gpt-4o-mini` costs ~$0.001-0.004 per request.

## Security Best Practices

### Protect Your API Key

- ✅ **Never commit API keys to version control**
- ✅ **Use environment variables**
- ✅ **Rotate keys regularly**
- ✅ **Use separate keys for different environments**

### Example .gitignore entry:

```gitignore
# Environment files
.env
.env.local
*.key

# Config files with secrets
config.yaml
config.*.yaml
!config.*.yaml.example
```

## Troubleshooting

### Common Issues

#### "Incorrect API key provided"
- Verify the key is correctly set in your environment
- Check for extra spaces or characters
- Ensure the key starts with `sk-`

#### "You exceeded your current quota"
- Check your billing settings
- Add payment method if not already added
- Verify your usage limits

#### "Rate limit exceeded"
- Reduce the frequency of requests
- Implement retry logic with exponential backoff
- Consider upgrading to a higher tier plan

### Testing Your Setup

Test your API key with a simple curl command:

```bash
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

You should see a JSON response listing available models.

## Support

- [OpenAI API Documentation](https://platform.openai.com/docs)
- [OpenAI Community Forum](https://community.openai.com/)
- [OpenAI Help Center](https://help.openai.com/)

---

**Next Steps**: After setting up your OpenAI API key, return to the main [README](../README.md) to continue with AWS configuration.