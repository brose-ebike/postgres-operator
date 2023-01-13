#!/bin/bash
import os
import sys

format = len(sys.argv) > 1 and (sys.argv[1] == "format")

with open("hack/boilerplate.go.txt", encoding="utf-8") as fp:
    header = fp.read()

source_files=[]
for parent,_,files in os.walk("."):
    if parent.startswith("./.git/"):
        continue
    for file in files:
        if not file.endswith(".go"):
            continue
        source_files.append(os.path.join(parent, file))


for file in source_files:
    with open(file, encoding="utf-8") as fp:
        content = fp.read()
    if content.contains(header):
        continue
    if format:
        with open(file, "w", encoding="utf-8") as fp:
            fp.write(header)
            fp.write("\n")
            fp.write(content)
    print(file, "does not start with header")