"""
Setup script for diskworker Python SDK
"""

import os
import pprint
import sys
import tomllib

from setuptools import setup, find_packages

DEBUG = False

if not os.path.isfile("pyproject.toml"):
    print("Error: pyproject.toml file not found", file=sys.stderr)
    sys.exit(1)

with open("pyproject.toml", "rb") as f:
    pyproject_data = tomllib.load(f)

project_data = pyproject_data.get("project", {})
poetry_data = pyproject_data.get("tool", {}).get("poetry", {})

# 메타데이터 소스 결정 (project 섹션 우선, 없으면 poetry 섹션 사용)
metadata_source = project_data if project_data else poetry_data

name = metadata_source.get("name", "")
version = metadata_source.get("version", "0.0.0")
description = metadata_source.get("description", "")
readme = metadata_source.get("readme", "README.md")
requires_python = metadata_source.get("requires-python", ">=3.7")
license_data = metadata_source.get("license", {})
license_info = license_data if isinstance(license_data, str) else license_data.get("text", "")
authors = metadata_source.get("authors", [])
maintainers = metadata_source.get("maintainers", [])

packages_find = pyproject_data.get("tool", {}).get("setuptools", {}).get("packages", {}).get("find", {})

dependencies = metadata_source.get("dependencies", [])
optional_dependencies = metadata_source.get("optional-dependencies", {})

try:
    with open(os.path.join(os.path.dirname(__file__), readme), "r", encoding="utf-8") as f:
        long_description = f.read()
except FileNotFoundError:
    long_description = description

install_requires = []
if isinstance(dependencies, list):
    install_requires = dependencies
elif isinstance(dependencies, dict):
    for package, version_spec in dependencies.items():
        if isinstance(version_spec, str):
            install_requires.append(f"{package}{version_spec}")
        else:
            install_requires.append(package)

extras_require = {}
for extra_name, extra_deps in optional_dependencies.items():
    if isinstance(extra_deps, list):
        extras_require[extra_name] = extra_deps
    elif isinstance(extra_deps, dict):
        extras_require[extra_name] = [
            f"{pkg}{ver}" if isinstance(ver, str) else pkg
            for pkg, ver in extra_deps.items()
        ]

author = ", ".join([author["name"] for author in authors]) if authors else ""
author_email = ", ".join([author["email"] for author in authors]) if authors else ""

urls = metadata_source.get("urls", {})
homepage = urls.get("Homepage", "")

classifiers = metadata_source.get("classifiers", [])

setup_kwargs = {
    "name": name,
    "version": version,
    "description": description,
    "long_description": long_description,
    "long_description_content_type": "text/markdown",
    "author": author,
    "author_email": author_email,
    "python_requires": requires_python,
    "license": license_info,
    "packages": find_packages(
        where = packages_find["where"][0],
        include=packages_find["include"],
        exclude=packages_find["exclude"],
    ),
    "install_requires": install_requires,
    "extras_require": extras_require,
    "classifiers": classifiers,
    "url": homepage,
    "project_urls": {k: v for k, v in urls.items()},
}

if DEBUG:
    pp = pprint.PrettyPrinter(indent=2)
    pp.pprint(setup_kwargs)

setup(**setup_kwargs)
