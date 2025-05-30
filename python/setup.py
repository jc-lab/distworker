"""
Setup script for DistWorker Python SDK
"""

from setuptools import setup, find_packages
import os

# Read README file
def read_readme():
    readme_path = os.path.join(os.path.dirname(__file__), 'README.md')
    if os.path.exists(readme_path):
        with open(readme_path, 'r', encoding='utf-8') as f:
            return f.read()
    return "DistWorker Python SDK for connecting workers to the DistWorker distributed task processing system."

# Read requirements
def read_requirements():
    requirements_path = os.path.join(os.path.dirname(__file__), 'requirements.txt')
    if os.path.exists(requirements_path):
        with open(requirements_path, 'r', encoding='utf-8') as f:
            return [line.strip() for line in f if line.strip() and not line.startswith('#')]
    return []

setup(
    name="distworker-sdk",
    version="1.0.0",
    author="JC-Lab",
    author_email="jc@jc-lab.net",
    description="Python SDK for DistWorker distributed task processing system",
    long_description=read_readme(),
    long_description_content_type="text/markdown",
    url="https://github.com/jc-lab/distworker",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: System :: Distributed Computing",
    ],
    python_requires=">=3.8",
    install_requires=read_requirements(),
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-asyncio>=0.21.0",
            "black>=23.0.0",
            "flake8>=6.0.0",
            "mypy>=1.0.0",
        ],
        "examples": [
            "aiohttp>=3.8.0",
            "aiofiles>=23.0.0",
            "Pillow>=10.0.0",
        ]
    },
    entry_points={
        "console_scripts": [
            "distworker-basic=distworker.examples.basic_worker:main",
            "distworker-file=distworker.examples.file_worker:main",
        ],
    },
    include_package_data=True,
    package_data={
        "distworker": ["py.typed"],
    },
    zip_safe=False,
    keywords="distributed, worker, task, processing, websocket, protobuf",
    project_urls={
        "Bug Reports": "https://github.com/jc-lab/distworker/issues",
        "Source": "https://github.com/jc-lab/distworker",
        "Documentation": "https://github.com/jc-lab/distworker/blob/main/README.md",
    },
)