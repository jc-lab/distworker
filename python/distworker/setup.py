from setuptools import setup, find_packages

setup(
    name='distworker',
    version='0.0.1',
    description='distworker worker sdk',
    author='jc-lab',
    author_email='joseph@jc-lab.net',
    url='https://github.com/jc-lab/distworker',
    install_requires=[
        'websockets',
        'protobuf',
        'googleapis-common-protos',
        'psutil'
    ],
    packages=find_packages(exclude=[]),
    keywords=['distworker'],
    python_requires='>=3.0',
    package_data={},
    zip_safe=False,
    classifiers=[],
)
