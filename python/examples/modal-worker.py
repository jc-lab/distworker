import os

import modal

app = modal.App("distworker-sample-worker")
worker_image = modal.Image.debian_slim().pip_install(
    "distworker-sdk==0.0.2rc1",
    "protobuf>=6.31.1",
)

@app.function(
    image=worker_image,
    timeout=1800
)
async def worker(controller_url: str, provisioner: str, worker_id: str, worker_token: str):
    import asyncio
    from typing import Dict, Any
    from distworker import Worker, Task

    worker = Worker(
        controller_url=controller_url,
        provisioner=provisioner,
        worker_id=worker_id,
        worker_token=worker_token,
        reconnect_interval=5.0,
        heartbeat_interval=5.0
    )

    async def task_handler(task: Task) -> Dict[str, Any]:
        print("task_handler: ", task)
        output_message = task.get_input("message") * 10

        await worker.send_task_progress(30.0, "processing")

        # Simulate some processing time
        await asyncio.sleep(1)

        await worker.send_task_progress(90.0, "finishing")

        # Simulate some processing time
        await asyncio.sleep(1)

        return {
            "message": output_message,
        }

    worker.task_handler = task_handler

    print(f"worker start: controller_url={controller_url}, worker_id={worker_id}")
    await worker.run()

@app.local_entrypoint()
def main():
    controller_url = os.environ['DISTWORKER_CONTROLLER_URL']
    provisioner = os.getenv('DISTWORKER_PROVISIONER', 'modal')
    worker_id = os.environ['DISTWORKER_WORKER_ID']
    worker_token = os.environ['DISTWORKER_WORKER_TOKEN']
    worker.remote(controller_url, provisioner, worker_id, worker_token)
