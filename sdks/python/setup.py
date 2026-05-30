#!/usr/bin/env python3
"""Setup script for the Sandforge Python SDK."""

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="sandforge-sdk",
    version="0.1.1",
    author="Anurag Yadav",
    author_email="yadavanurag1310@gmail.com",
    description="Python SDK for Sandforge hypervisor sandbox platform",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/yanurag-dev/sandforge",
    packages=find_packages(),
    package_data={"sandforge": ["py.typed"]},
    classifiers=[
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "License :: OSI Approved :: Apache Software License",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.33.0",
    ],
    extras_require={
        "dev": [
            "pytest>=6.0",
            "pytest-cov>=2.10",
            "black>=21.0",
            "flake8>=3.9",
            "mypy>=0.910",
        ],
    },
)
