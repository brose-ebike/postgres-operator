with open("version.txt") as fp:
    version = fp.read()

parts = version.split(".")
parts[-1] = str(int(parts[-1]) + 1)
version = ".".join(parts)

with open("version.txt", "w") as fp:
    fp.write(version)