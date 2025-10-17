# Google Gemini API Key Setup Guide

This guide will help you obtain a Google Gemini API key for use with the AI Infrastructure Agent.

## Prerequisites

- Google account
- Access to Google AI Studio

## Step-by-Step Instructions

### 1. Access Google AI Studio

1. Visit [Google AI Studio](https://aistudio.google.com/)
2. Sign in with your Google account
3. Accept the Terms of Service if prompted

### 2. Create API Key

1. In Google AI Studio, look for the **"Get API key"** button in the left sidebar
2. Click **"Create API key"**
3. Choose your project setup:
   - **Create API key in new project**: Creates a new Google Cloud project (recommended for beginners)
   - **Create API key in existing project**: Select from your existing Google Cloud projects
4. Copy the generated API key immediately and store it securely
5. **Important**: You won't be able to view the full key again after this step

### Alternative: Google Cloud Console Method

For enterprise use or more control:
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create or select a project
3. Enable the "Gemini API" 
4. Go to "Credentials" → "Create Credentials" → "API Key"
5. Restrict the API key to "Gemini API" for security

### 3. Choose Your Model

Available Gemini models for infrastructure tasks:

| Model | Best For | Speed | Capabilities |
|-------|----------|-------|-------------|
| `gemini-1.5-pro-latest` | Complex reasoning, production-ready | Medium | 1M token context, most reliable |
| `gemini-1.5-flash-latest` | Fast responses, balanced performance | Fast | 1M token context, cost-effective |
| `gemini-1.0-pro` | Legacy stable model | Medium | Standard context, deprecated |

**Recommended**: Use `gemini-1.5-pro-latest` for production infrastructure tasks, or `gemini-1.5-flash-latest` for development and testing.

### 4. Configure Google Cloud (Optional but Recommended)

For production use, it's recommended to set up proper Google Cloud integration:

1. Visit [Google Cloud Console](https://console.cloud.google.com/)
2. Create or select a project
3. Enable the **Gemini API**
4. Set up billing (pay-per-use pricing)
5. Configure quotas and limits

### 5. Configure the Agent

Set your API key as an environment variable:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export GEMINI_API_KEY="your-api-key-here"

# Or set it temporarily for the current session
export GEMINI_API_KEY="your-api-key-here"
```

Update your `config.yaml`:

```yaml
agent:
  provider: "gemini"
  model: "gemini-1.5-flash-latest"    # Recommended for balanced performance and cost
  max_tokens: 4000
  temperature: 0.1
```

## Usage and Pricing

### Free Tier Limits

Google AI Studio provides a free tier for the Gemini API. The limits may vary, but typically include a generous number of requests per minute and a substantial monthly quota suitable for development and testing. For the most up-to-date information, please refer to the official [Google AI pricing page](https://ai.google.dev/pricing).

### Current Pricing (USD per 1M tokens)

| Model | Input Tokens | Output Tokens |
|-------|-------------|---------------|
| `gemini-1.5-pro-latest` | $3.50 | $10.50 |
| `gemini-1.5-flash-latest` | $0.35 | $1.05 |

💡 **Tip**: Infrastructure tasks typically cost $0.0002-0.003 per request with `gemini-1.5-flash-latest`.

### Paid Usage

When you exceed free limits:

- **Pay-per-use**: Only pay for what you consume
- **Competitive pricing**: Generally more cost-effective than OpenAI
- **Automatic scaling**: No pre-payment required
- **Volume discounts**: Available for high-usage scenarios

## Regional Availability

Gemini API is available globally with some regional considerations:

- ✅ **Americas**: Full feature availability, all models supported
- ✅ **Europe**: Full feature availability, all models supported  
- ✅ **Asia-Pacific**: Full feature availability, all models supported
- ⚠️ **Some regions**: Certain experimental models may have limited availability

Check [Google AI availability](https://ai.google.dev/available_regions) for the most current regional status and model availability.

## Security Best Practices

### Protect Your API Key

- ✅ **Never commit API keys to version control**
- ✅ **Use environment variables**
- ✅ **Rotate keys regularly**
- ✅ **Use separate keys for different environments**

### Environment Security

```bash
# Example .env file (never commit this)
GEMINI_API_KEY=your-actual-key-here

# Add to .gitignore
echo ".env" >> .gitignore
echo "config.yaml" >> .gitignore
```

## Advanced Configuration

### Custom Settings

```yaml
agent:
  provider: "gemini"
  model: "gemini-1.5-pro"
  max_tokens: 8000
  temperature: 0.2
  # Gemini-specific settings
  top_p: 0.8
  top_k: 10
```

### Safety Settings

Gemini includes built-in safety features:

- **Content filtering**: Automatic harmful content detection
- **Safety categories**: Harassment, hate speech, sexually explicit, dangerous content
- **Configurable thresholds**: Adjust sensitivity levels

## Troubleshooting

### Common Issues

#### "API key not valid"
```bash
# Verify your key is set correctly
echo $GEMINI_API_KEY

# Test with curl
curl -H "Content-Type: application/json" \
     -d '{"contents":[{"parts":[{"text":"Hello"}]}]}' \
     "https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent?key=$GEMINI_API_KEY"
```

#### "Quota exceeded"
- Check your usage in Google AI Studio
- Upgrade to paid tier if needed
- Implement rate limiting in your application

#### "Model not found"
- Verify model name spelling
- Check model availability in your region
- Use a supported model variant

#### "Permission denied"
- Ensure API is enabled in Google Cloud Console
- Check IAM permissions if using service accounts
- Verify billing account is active

### Testing Your Setup

Test your Gemini API key:

```bash
# Simple test request
curl -H "Content-Type: application/json" \
     -d '{
       "contents": [{
         "parts": [{"text": "Respond with: API key working correctly"}]
       }]
     }' \
     "https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent?key=$GEMINI_API_KEY"
```

Successful response should include generated text.

## Performance Tips

### Optimize for Infrastructure Tasks

- **Use specific models**: `gemini-1.5-pro` for complex planning
- **Adjust temperature**: Lower values (0.1-0.3) for consistent outputs
- **Set appropriate token limits**: 4000-8000 for infrastructure tasks
- **Use system instructions**: Provide context about AWS infrastructure

### Example Configuration for Infrastructure Tasks

```yaml
agent:
  provider: "gemini"
  model: "gemini-1.5-flash-latest"    # Updated model with better performance
  max_tokens: 6000
  temperature: 0.15
  dry_run: true
  enable_debug: false
```

**Alternative configurations:**

```yaml
# For cost-sensitive development
agent:
  provider: "gemini"
  model: "gemini-1.5-flash-latest"    # Most cost-effective
  max_tokens: 4000
  temperature: 0.1

# For complex infrastructure planning
agent:
  provider: "gemini" 
  model: "gemini-1.5-pro-latest"     # Most capable for complex tasks
  max_tokens: 8000
  temperature: 0.1
```

## Support and Resources

- [Google AI Studio](https://aistudio.google.com/)
- [Gemini API Documentation](https://ai.google.dev/docs)
- [Google AI Developer Community](https://developers.googleblog.com/2023/12/how-its-made-gemini-multimodal-prompting.html)
- [Pricing Information](https://ai.google.dev/pricing)

---

**Next Steps**: After setting up your Gemini API key, return to the main [README](../README.md) to continue with AWS configuration.