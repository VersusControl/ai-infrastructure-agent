# Ollama Local LLM Setup Guide

This guide will help you set up Ollama for local LLM inference with the AI Infrastructure Agent.

## What is Ollama?

Ollama is a tool that allows you to run large language models locally on your machine. It provides:
- **Privacy**: All inference happens on your machine - no data sent to external APIs
- **No API Costs**: Free to use once you have the hardware
- **Offline Capability**: Works without internet connection
- **Fast Inference**: Optimized for local hardware (CPU/GPU)

## Prerequisites

- **Operating System**: macOS, Linux, or Windows (WSL2)
- **Hardware Requirements**:
  - **Minimum**: 8GB RAM (for small models like Llama 3.2 3B)
  - **Recommended**: 16GB+ RAM (for larger models like Llama 3.3 70B)
  - **GPU**: Optional but recommended (NVIDIA, AMD, or Apple Silicon)
- **Disk Space**: 4-40GB depending on model size

## Step-by-Step Instructions

### Option 1: Local Model Setup (Recommended for Getting Started)

This is the simplest way to get started with Ollama on your local machine.

#### 1. Install Ollama

**macOS / Linux:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**macOS (Homebrew):**
```bash
brew install ollama
```

**Windows (WSL2):**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**Manual Installation:** Download from [https://ollama.com/download](https://ollama.com/download)

#### 2. Pull and Run a Model

For most infrastructure tasks, start with `gemma3:27b`:

```bash
# Pull the model (downloads to your machine)
ollama pull gemma3:27b

# Test the model interactively
ollama run gemma3:27b
```

This will start an interactive chat. Type `/bye` to exit.

**Quick test:**
```bash
ollama run gemma3:27b "What is AWS EC2?"
```

**Other recommended models:**
```bash
# Faster, smaller model (3B parameters)
ollama pull llama3.2

# Latest Gemma 3 (4B parameters, good balance)
ollama pull gemma3

# Larger, more capable (70B parameters)
ollama pull llama3.3

# View all downloaded models
ollama list
```

#### 3. Configure Your Agent

Copy the example config and edit it:

```bash
cp config.ollama.yaml.example config.yaml
```

Edit `config.yaml`:

```yaml
agent:
  provider: "ollama"
  model: "gemma3:27b"        # or "gemma3", "llama3.2", etc.
  max_tokens: 8192
  temperature: 0.1
```

That's it! Ollama runs on `http://localhost:11434` by default (no additional configuration needed).

#### 4. Start the Agent

```bash
# Option 1: Using the launch script
./scripts/run-web-ui.sh
```

Open your browser and navigate to:
```
http://localhost:8080
```

---

### Option 2: Remote Ollama Server Setup

Use this if you want to run Ollama on a different machine (e.g., a more powerful server).

#### On the Remote Server:

**1. Install and start Ollama with network access:**
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama to accept connections from any IP
OLLAMA_HOST=0.0.0.0:11434 ollama serve
```

**2. Pull models on the server:**
```bash
ollama pull gemma3:27b
ollama pull gemma3
```

**3. Make sure port 11434 is open:**
```bash
# Check if reachable (from client machine)
curl http://your-server-ip:11434/api/version
```

#### On Your Local Machine (Agent):

**Configure the agent to use remote server:**

Edit `config.yaml`:

```yaml
agent:
  provider: "ollama"
  ollama_server_url: "http://your-server-ip:11434"  # Remote Ollama server
  model: "gemma3:27b"
  max_tokens: 8192
  temperature: 0.1
```

Or use environment variable:
```bash
export OLLAMA_SERVER_URL="http://your-server-ip:11434"
./scripts/run-web-ui.sh
```

---

### Quick Start Summary

**Local (simplest):**
```bash
# 1. Install
curl -fsSL https://ollama.com/install.sh | sh

# 2. Pull & test model
ollama pull gemma3:27b
ollama run gemma3:27b

# 3. Configure
cp config.ollama.yaml.example config.yaml
# Edit config.yaml: set provider="ollama", model="gemma3:27b"

# 4. Run
./scripts/run-web-ui.sh
```

**Remote:**
```bash
# On server: 
OLLAMA_HOST=0.0.0.0:11434 ollama serve
ollama pull gemma3:27b

# On client (edit config.yaml):
# ollama_server_url: "http://server-ip:11434"
./scripts/run-web-ui.sh
```

## Model Comparison

### Recommended Models for Infrastructure Tasks

| Model | Size | RAM Needed | Speed | Capability | Best For |
|-------|------|-----------|-------|------------|----------|
| `gemma3:27b` | 7B | 8GB | ⚡⚡ | ⭐⭐⭐ | **Getting Started** - Reliable, well-tested |
| `gemma3` | 4B | 8GB | ⚡⚡⚡ | ⭐⭐⭐ | Fast and efficient, good balance |
| `llama3.2` | 3B | 8GB | ⚡⚡⚡ | ⭐⭐ | Fastest, good for simple tasks |
| `llama3.3` | 70B | 64GB | ⚡ | ⭐⭐⭐⭐⭐ | Most capable, complex tasks |
| `deepseek-r1` | 7B | 16GB | ⚡⚡ | ⭐⭐⭐⭐ | Reasoning and planning tasks |
| `qwq` | 32B | 32GB | ⚡ | ⭐⭐⭐⭐ | Advanced reasoning |

**Pull any model:**
```bash
ollama pull <model-name>  # e.g., ollama pull gemma3:27b
```

### Context Window Sizes

| Model | Default Context | Recommended Setting |
|-------|----------------|---------------------|
| Llama 2 | 4K tokens | 4096-8192 |
| Gemma 3 | 8K tokens | 8192 |
| Llama 3.2/3.3 | 128K tokens | 8192-16384 |
| DeepSeek R1 | 64K tokens | 8192-16384 |

**Recommendation**: Start with `max_tokens: 8192` - sufficient for most infrastructure tasks.

## Advanced Configuration

### GPU Acceleration

Ollama automatically uses GPU if available (no configuration needed).

**Check GPU usage:**
```bash
# NVIDIA
nvidia-smi

# Apple Silicon - check Activity Monitor > GPU tab
```

**Note:** GPU acceleration happens automatically. Most models will run faster on GPU.

## Troubleshooting

### Common Issues

1. **"Connection refused" or "Cannot connect to Ollama"**
   - Check if Ollama is running: `curl http://localhost:11434/api/version`
   - Start Ollama: `ollama serve`
   - Check firewall settings

2. **"Model not found"**
   - Pull the model first: `ollama pull <model-name>`
   - List available models: `ollama list`
   - Use exact model name with tag (e.g., `llama3.3:latest`)

3. **Out of Memory errors**
   - Use a smaller model
   - Reduce `max_tokens` in config
   - Close other applications
   - Reduce `num_gpu` layers

4. **Slow responses**
   - Use smaller model for faster inference
   - Enable GPU acceleration
   - Reduce context window size
   - Increase `num_thread` for CPU inference

5. **Model pulling fails**
   - Check internet connection
   - Check disk space: `df -h`
   - Try again: `ollama pull <model-name>`
   - Clear cache: `ollama rm <model-name>` then pull again

### Getting Help

- **Ollama Documentation**: [https://github.com/ollama/ollama](https://github.com/ollama/ollama)
- **Ollama Discord**: [https://discord.gg/ollama](https://discord.gg/ollama)
- **Ollama GitHub Issues**: [https://github.com/ollama/ollama/issues](https://github.com/ollama/ollama/issues)
- **Model Library**: [https://ollama.com/library](https://ollama.com/library)

## Comparison with Cloud LLMs

### Advantages of Ollama:
- ✅ **Privacy**: Data stays on your machine
- ✅ **No API costs**: Free after initial setup
- ✅ **Offline**: Works without internet
- ✅ **Low latency**: Fast local inference
- ✅ **Customizable**: Full control over parameters

### Advantages of Cloud LLMs:
- ✅ **No hardware requirements**: Works on any device
- ✅ **Latest models**: Access to newest, most capable models
- ✅ **Scalability**: Handles any load
- ✅ **No maintenance**: Managed service

### When to Use Ollama:
- Testing and development
- Privacy-sensitive workloads
- Cost optimization for high-volume usage
- Offline environments
- Learning and experimentation

### When to Use Cloud LLMs:
- Production deployments
- Need for most capable models
- Limited local hardware
- Variable workload
- Multi-user scenarios

## Best Practices

1. **Start Small**: Begin with smaller models (3-8B) for testing
2. **Monitor Resources**: Keep an eye on RAM and CPU/GPU usage
3. **Version Control**: Use specific model tags (`:latest`, `:70b`, etc.)
4. **Regular Updates**: Update Ollama and models periodically
5. **Backup Models**: Keep important model files backed up
6. **Temperature Setting**: Use 0.1-0.3 for infrastructure tasks
7. **Context Size**: Start with 8K, increase only if needed

## Security Considerations

- **Local Only**: Ollama runs locally - no data sent externally
- **Network Access**: Be careful when exposing Ollama to network
- **Model Source**: Only pull models from trusted sources
- **Access Control**: Use firewalls if running as network service
- **Resource Limits**: Set memory limits to prevent system overload

## Next Steps

- [Getting Started Guide](/getting-started.md)
- [Architecture Overview](/architecture/architecture-overview.md)
- [Working with EC2 Instances](/examples/working-with-ec2-instance.md)
- [Working with VPC](/examples/working-with-vpc.md)
