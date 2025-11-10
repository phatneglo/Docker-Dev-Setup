# Best Model Download Guide for RTX 4070 Ti

## 🏆 Recommended: Llama 3.1 8B AWQ (4-bit)

**Why AWQ is the best:**
- ✅ **Best quality/speed balance** - Minimal quality loss vs FP16
- ✅ **Lower memory usage** - ~4-5GB VRAM (vs 5-6GB for FP8)
- ✅ **Faster inference** - Optimized for vLLM
- ✅ **More headroom** - Leaves 7GB free for longer contexts
- ✅ **Better performance** - 15-25 tokens/s generation

## 📥 Download Instructions

### Step 1: Install HuggingFace CLI
```bash
pip install huggingface-hub
```

### Step 2: Download the Model

**Option A: Download to your models folder (Recommended)**
```bash
huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ --local-dir C:\models\llama3.1-8b-awq
```

**Option B: Download with resume support (if interrupted)**
```bash
huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ --local-dir C:\models\llama3.1-8b-awq --resume-download
```

**Option C: Download specific files only (faster, smaller)**
```bash
# Download only essential files (saves time and space)
huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ \
  --local-dir C:\models\llama3.1-8b-awq \
  --include "*.safetensors" "*.json" "tokenizer*" "*.txt"
```

### Step 3: Verify Download
Check that these files exist in `C:\models\llama3.1-8b-awq`:
- `config.json`
- `tokenizer.json` or `tokenizer.model`
- `*.safetensors` files (model weights)
- `quantization_config.json` (AWQ config)

## 🔧 Update docker-compose.yml

After downloading, update your configuration:

```yaml
volumes:
  - C:\models\llama3.1-8b-awq:/model  # Change from llama3.1-8b-fp8

command:
  - --model
  - /model
  - --quantization
  - awq  # Add this line
  - --served-model-name
  - llama-3.1-8b
  - --max-model-len
  - "8192"  # Can increase to 8K with AWQ!
  - --gpu-memory-utilization
  - "0.60"
  # ... rest of config
```

## 📊 Model Comparison

| Model | VRAM | Quality | Speed | Context | Best For |
|-------|------|---------|-------|---------|----------|
| **8B AWQ (4-bit)** | 4-5GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 8K+ | **Best overall** |
| 8B GPTQ (4-bit) | 4-5GB | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 8K+ | Alternative |
| 8B FP8 | 5-6GB | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 4K-6K | Current |
| 13B AWQ (4-bit) | 7-8GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 4K-6K | Better quality |

## 🚀 Alternative: 13B AWQ (If You Want Better Quality)

If you want superior quality and can spare more VRAM:

```bash
huggingface-cli download TheBloke/Llama-3.1-13B-Instruct-AWQ --local-dir C:\models\llama3.1-13b-awq
```

**Then update docker-compose.yml:**
- Change volume to: `C:\models\llama3.1-13b-awq:/model`
- Reduce `max_model_len` to `6144` or `4096`
- Keep `gpu_memory_utilization` at `0.60`

## 📋 Complete Setup Steps

1. **Download the model:**
   ```bash
   huggingface-cli download TheBloke/Llama-3.1-8B-Instruct-AWQ --local-dir C:\models\llama3.1-8b-awq
   ```

2. **Update docker-compose.yml** (see above)

3. **Restart container:**
   ```bash
   cd vllm
   docker-compose down
   docker-compose up -d
   ```

4. **Test:**
   ```bash
   php test_vllm.php
   ```

## 💾 Download Size

- **8B AWQ**: ~4.5GB (compressed) → ~5GB (extracted)
- **13B AWQ**: ~7GB (compressed) → ~8GB (extracted)

## ⚡ Expected Performance (8B AWQ)

- **Generation Speed**: 15-25 tokens/s
- **First Token Latency**: 50-100ms
- **Memory Usage**: 4-5GB VRAM
- **Context Length**: Up to 8K tokens comfortably
- **Quality**: Near FP16 quality (minimal degradation)

## 🔍 Verify Model Format

After download, check the model directory contains:
```
llama3.1-8b-awq/
├── config.json
├── quantization_config.json  ← AWQ config
├── tokenizer.json
├── tokenizer_config.json
├── model-00001-of-00002.safetensors
├── model-00002-of-00002.safetensors
└── ...
```

## 🎯 Why AWQ Over FP8?

1. **Better Memory Efficiency**: Uses 4-bit vs 8-bit
2. **Optimized for vLLM**: Native AWQ support
3. **Better Quality**: Activation-aware quantization preserves important weights
4. **More Headroom**: Leaves more VRAM for longer contexts
5. **Faster**: Optimized kernels for AWQ

## 🆘 Troubleshooting

**Download fails?**
- Check internet connection
- Use `--resume-download` flag
- Try downloading during off-peak hours

**Model not loading?**
- Verify `quantization_config.json` exists
- Check file permissions
- Ensure all `.safetensors` files downloaded

**Out of memory?**
- Reduce `max_model_len` to `4096`
- Lower `gpu_memory_utilization` to `0.55`

## 📚 Model Links

- **8B AWQ**: https://huggingface.co/TheBloke/Llama-3.1-8B-Instruct-AWQ
- **13B AWQ**: https://huggingface.co/TheBloke/Llama-3.1-13B-Instruct-AWQ
- **8B GPTQ**: https://huggingface.co/TheBloke/Llama-3.1-8B-Instruct-GPTQ

