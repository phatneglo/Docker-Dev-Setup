# vLLM Troubleshooting Guide

## Engine Core Initialization Failed

If you're still experiencing the "Engine core initialization failed" error, try these steps:

### 1. Verify GPU Access
```bash
docker run --rm --gpus all nvidia/cuda:12.0.0-base-ubuntu22.04 nvidia-smi
```

### 2. Check Model Path
Ensure the model directory exists and contains:
- `config.json`
- `tokenizer.json` or `tokenizer.model`
- Model weights (`.safetensors` or `.bin` files)

### 3. Verify Model Format
The model should be in HuggingFace format. If you have a different format, you may need to convert it.

### 4. Check GPU Memory
If you have limited GPU memory, try:
- Reduce `--gpu-memory-utilization` to `0.7` or `0.8`
- Reduce `--max-model-len` to `4096` or `2048`
- Use a smaller model variant

### 5. Check Container Logs
```bash
docker logs vllm
```

### 6. Alternative Configuration (CPU-only for testing)
If GPU issues persist, you can test with CPU (very slow):
```yaml
# Remove runtime: nvidia and deploy section
# Add: --device cpu
```

### 7. Common Issues

**Issue: CUDA out of memory**
- Solution: Reduce `--gpu-memory-utilization` or `--max-model-len`

**Issue: Model not found**
- Solution: Verify the volume mount path is correct and accessible

**Issue: Permission denied**
- Solution: Ensure Docker has access to the model directory

**Issue: Missing dependencies**
- Solution: Try pulling the latest image: `docker pull vllm/vllm-openai:latest`

### 8. Test with Minimal Configuration
Try starting with minimal settings:
```yaml
command:
  - --model
  - /model
  - --served-model-name
  - llama-3.1-8b
  - --gpu-memory-utilization
  - "0.7"
```

