# Best Models for RTX 4070 Ti (12GB VRAM)

## Current Setup
- **Model**: Llama 3.1 8B FP8
- **Status**: Working, but can be optimized

## Recommended Models (Best to Good)

### 🏆 Best Option: Llama 3.1 8B AWQ (4-bit)
**Why**: Best quality/speed/memory balance
- **VRAM Usage**: ~4-5GB
- **Quality**: Excellent (minimal quality loss)
- **Speed**: Very fast
- **Context**: Can handle 8K+ tokens easily
- **Download**: 
  ```bash
  # From HuggingFace
  huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ
  ```

### 🥈 Second Best: Llama 3.1 13B AWQ (4-bit)
**Why**: Better quality, still fits in 12GB
- **VRAM Usage**: ~7-8GB
- **Quality**: Superior to 8B
- **Speed**: Good (slightly slower than 8B)
- **Context**: Can handle 4K-6K tokens
- **Download**:
  ```bash
  huggingface-cli download TheBloke/Llama-3.1-13B-Instruct-AWQ
  ```

### 🥉 Third: Llama 3.1 8B GPTQ (4-bit)
**Why**: Alternative quantization, good performance
- **VRAM Usage**: ~4-5GB
- **Quality**: Very good
- **Speed**: Fast
- **Download**:
  ```bash
  huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-GPTQ
  ```

### Current: Llama 3.1 8B FP8
**Why**: You're using this now
- **VRAM Usage**: ~5-6GB
- **Quality**: Good
- **Speed**: Good
- **Note**: FP8 is newer but AWQ often performs better

## Quantization Comparison

| Format | Quality | Speed | Memory | Recommendation |
|--------|---------|-------|--------|----------------|
| **AWQ 4-bit** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | **Best overall** |
| **GPTQ 4-bit** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Good alternative |
| **FP8** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Current (good) |
| **FP16** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | Too large for 12GB |

## How to Switch Models

### Option 1: Update docker-compose.yml
```yaml
volumes:
  - C:\models\llama3.1-8b-awq:/model  # Change path to AWQ model
```

### Option 2: Use HuggingFace Model ID (if you have internet)
```yaml
command:
  - --model
  - TheBloke/Llama-3.1-8B-Instruct-AWQ
  - --quantization
  - awq
```

## Performance Optimization Tips

1. **Enable Prefix Caching**: Already added to config
2. **Use Swap Space**: Already added (4GB) for longer contexts
3. **Increase max_model_len**: Set to 8192 (already done)
4. **GPU Memory Utilization**: Set to 0.92 (already done)

## Expected Performance (8B AWQ)

- **Throughput**: 15-25 tokens/s
- **Latency**: 50-100ms first token
- **Context**: Up to 8K tokens comfortably
- **Quality**: Near FP16 quality

## Download Commands

### For AWQ (Recommended):
```bash
# Install huggingface-cli if needed
pip install huggingface-hub

# Download 8B AWQ
huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ --local-dir C:\models\llama3.1-8b-awq

# Download 13B AWQ (if you want better quality)
huggingface-cli download TheBloke/Llama-3.1-13B-Instruct-AWQ --local-dir C:\models\llama3.1-13b-awq
```

### For GPTQ:
```bash
huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-GPTQ --local-dir C:\models\llama3.1-8b-gptq
```

## vLLM Configuration for AWQ

If using AWQ, add quantization flag:
```yaml
command:
  - --model
  - /model
  - --quantization
  - awq
  - --served-model-name
  - llama-3.1-8b
  # ... rest of config
```

## Testing Your Model

After switching, test with:
```bash
php test_vllm.php
```

Monitor GPU usage:
```bash
docker exec vllm nvidia-smi
```

