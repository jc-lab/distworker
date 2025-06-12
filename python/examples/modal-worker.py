import os
import time
import uuid
from typing import List, Optional

import modal

app = modal.App("distworker-sample-worker")
worker_image = modal.Image.debian_slim().pip_install(
    "distworker-sdk==0.0.3rc2",
    "protobuf>=4.31.1",
    "transformers>=4.35.0",
    "torch>=2.1.0",
    "accelerate>=0.24.0",
    "sentencepiece>=0.1.99",
    "tiktoken>=0.5.0",
)

@app.function(
    image=worker_image,
    timeout=1800,
    gpu="T4",
    memory=8192
)
async def worker(controller_url: str, provisioner: str, worker_id: str, worker_token: str):
    from typing import Dict, Any
    from distworker import Worker, Request
    import torch
    from transformers import AutoTokenizer, AutoModelForCausalLM
    import tiktoken

    # 가벼운 모델 로드 (GPT-2 또는 다른 소형 모델)
    print("Loading model...")
    model_name = "microsoft/DialoGPT-small"  # 대화형 모델
    tokenizer = AutoTokenizer.from_pretrained(model_name, padding_side='left')
    model = AutoModelForCausalLM.from_pretrained(
        model_name,
        torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
        device_map="auto" if torch.cuda.is_available() else None
    )

    # pad token 설정
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    print("Model loaded successfully!")

    def count_tokens(text: str) -> int:
        """토큰 수 계산"""
        try:
            # tiktoken을 사용해 대략적인 토큰 수 계산
            encoding = tiktoken.get_encoding("cl100k_base")
            return len(encoding.encode(text))
        except:
            # fallback: 단어 수 기반 추정
            return len(text.split()) * 1.3

    def generate_text(prompt: str, max_tokens: int = 100, temperature: float = 0.7,
                      stop_sequences: Optional[List[str]] = None) -> str:
        """텍스트 생성"""
        try:
            inputs = tokenizer.encode(prompt, return_tensors="pt")
            if torch.cuda.is_available():
                inputs = inputs.cuda()

            with torch.no_grad():
                outputs = model.generate(
                    inputs,
                    max_new_tokens=min(max_tokens, 512),  # 최대 토큰 제한
                    temperature=max(temperature, 0.1),
                    do_sample=True,
                    pad_token_id=tokenizer.pad_token_id,
                    eos_token_id=tokenizer.eos_token_id,
                    attention_mask=torch.ones_like(inputs)
                )

            generated_text = tokenizer.decode(outputs[0], skip_special_tokens=True)

            # 원본 프롬프트 제거
            if generated_text.startswith(prompt):
                generated_text = generated_text[len(prompt):].strip()

            # stop sequences 처리
            if stop_sequences:
                for stop_seq in stop_sequences:
                    if stop_seq in generated_text:
                        generated_text = generated_text.split(stop_seq)[0]

            return generated_text

        except Exception as e:
            print(f"Generation error: {e}")
            return "I apologize, but I encountered an error while generating a response."

    def handle_chat_completions(req: Request, data: Dict[str, Any]) -> Dict[str, Any]:
        """Chat completions API 처리"""
        messages = data.get("messages", [])
        max_tokens = data.get("max_tokens", 100)
        temperature = data.get("temperature", 0.7)
        stream = data.get("stream", False)
        model_name = data.get("model", "dialogpt-small")
        stop = data.get("stop", [])

        # 메시지를 프롬프트로 변환
        prompt = ""
        for message in messages:
            role = message.get("role", "user")
            content = message.get("content", "")

            if role == "system":
                prompt += f"System: {content}\n"
            elif role == "user":
                prompt += f"Human: {content}\n"
            elif role == "assistant":
                prompt += f"Assistant: {content}\n"

        prompt += "Assistant: "

        # 텍스트 생성
        generated_text = generate_text(
            prompt,
            max_tokens=max_tokens,
            temperature=temperature,
            stop_sequences=stop if isinstance(stop, list) else ([stop] if stop else [])
        )

        # 응답 생성
        response_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
        created = int(time.time())

        prompt_tokens = count_tokens(prompt)
        completion_tokens = count_tokens(generated_text)
        total_tokens = prompt_tokens + completion_tokens

        if stream:
            # 스트리밍 응답
            words = generated_text.split()
            for i, word in enumerate(words):
                chunk = {
                    "id": response_id,
                    "object": "chat.completion.chunk",
                    "created": created,
                    "model": model_name,
                    "choices": [{
                        "index": 0,
                        "delta": {
                            "content": word + (" " if i < len(words) - 1 else "")
                        },
                        "finish_reason": None
                    }]
                }
                req.progress(data = chunk)

            # 마지막 청크
            final_chunk = {
                "id": response_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model_name,
                "choices": [{
                    "index": 0,
                    "delta": {},
                    "finish_reason": "stop"
                }]
            }
            req.progress(data = final_chunk)

            return {}
        else:
            # 일반 응답
            return {
                "id": response_id,
                "object": "chat.completion",
                "created": created,
                "model": model_name,
                "choices": [{
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": generated_text
                    },
                    "finish_reason": "stop"
                }],
                "usage": {
                    "prompt_tokens": prompt_tokens,
                    "completion_tokens": completion_tokens,
                    "total_tokens": total_tokens
                }
            }

    def handle_completions(req: Request, data: Dict[str, Any]) -> Dict[str, Any]:
        """Text completions API 처리"""
        prompt = data.get("prompt", "")
        max_tokens = data.get("max_tokens", 100)
        temperature = data.get("temperature", 0.7)
        stream = data.get("stream", False)
        model_name = data.get("model", "dialogpt-small")
        stop = data.get("stop", [])
        n = data.get("n", 1)

        response_id = f"cmpl-{uuid.uuid4().hex[:29]}"
        created = int(time.time())

        choices = []

        for i in range(n):
            generated_text = generate_text(
                prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                stop_sequences=stop if isinstance(stop, list) else ([stop] if stop else [])
            )

            if stream:
                # 스트리밍 응답
                words = generated_text.split()
                for j, word in enumerate(words):
                    chunk = {
                        "id": response_id,
                        "object": "text_completion",
                        "created": created,
                        "model": model_name,
                        "choices": [{
                            "text": word + (" " if j < len(words) - 1 else ""),
                            "index": i,
                            "logprobs": None,
                            "finish_reason": None
                        }]
                    }
                    req.progress(data = chunk)

                # 마지막 청크
                final_chunk = {
                    "id": response_id,
                    "object": "text_completion",
                    "created": created,
                    "model": model_name,
                    "choices": [{
                        "text": "",
                        "index": i,
                        "logprobs": None,
                        "finish_reason": "stop"
                    }]
                }
                req.progress(data = final_chunk)
            else:
                choices.append({
                    "text": generated_text,
                    "index": i,
                    "logprobs": None,
                    "finish_reason": "stop"
                })

        if stream:
            return {}
        else:
            prompt_tokens = count_tokens(prompt)
            completion_tokens = sum(count_tokens(choice["text"]) for choice in choices)
            total_tokens = prompt_tokens + completion_tokens

            return {
                "id": response_id,
                "object": "text_completion",
                "created": created,
                "model": model_name,
                "choices": choices,
                "usage": {
                    "prompt_tokens": prompt_tokens,
                    "completion_tokens": completion_tokens,
                    "total_tokens": total_tokens
                }
            }

    worker = Worker(
        controller_url=controller_url,
        provisioner=provisioner,
        worker_id=worker_id,
        worker_token=worker_token,
        reconnect_interval=5.0,
        heartbeat_interval=5.0
    )

    async def task_handler(req: Request) -> Dict[str, Any]:
        task = req.task
        print(f"Processing task: queue={task.queue}, task_id={task.task_id}")

        try:
            if task.queue == "example/chat":
                return handle_chat_completions(req, task.input_data)
            elif task.queue == "example/completions":
                return handle_completions(req, task.input_data)
            else:
                error_msg = f"Unknown queue: {task.queue}"
                print(error_msg)
                return {
                    "error": {
                        "message": error_msg,
                        "type": "invalid_request_error",
                        "code": "unknown_queue"
                    }
                }
        except Exception as e:
            error_msg = f"Task processing error: {str(e)}"
            print(error_msg)
            return {
                "error": {
                    "message": error_msg,
                    "type": "internal_server_error",
                    "code": "processing_error"
                }
            }

    worker.task_handler = task_handler

    print(f"Worker started: controller_url={controller_url}, worker_id={worker_id}")
    await worker.run()

@app.local_entrypoint()
def main():
    controller_url = os.environ['DISTWORKER_CONTROLLER_URL']
    provisioner = os.getenv('DISTWORKER_PROVISIONER', 'modal')
    worker_id = os.environ['DISTWORKER_WORKER_ID']
    worker_token = os.environ['DISTWORKER_WORKER_TOKEN']
    worker.remote(controller_url, provisioner, worker_id, worker_token)