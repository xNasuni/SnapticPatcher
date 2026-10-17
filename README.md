>[!WARNING]
> superseded by [gdpatch](https://gdpatch.dev) + [SnapticBase](https://github.com/xNasuni/SnapticBase)<br>
> <ins>dont use this anymore</ins>

# SnapticPatcher

unofficial modding framework for the SNAPTIC demo

this doesn't distribute any game files

very early & only lightly tested

# Using mods

download the patcher from [the latest release](https://github.com/xNasuni/SnapticPatcher/releases/latest)

write the 64 character hex aes decryption key into key.txt file in the game directory

run `snaptic-patcher.exe` every time you want to patch the game with the new mods

# Creating Mods

see [the developer wiki](https://github.com/xNasuni/SnapticPatcher/wiki)

### Building

```
cd patcher
GOOS=windows go build -ldflags "-s -w" -o snaptic-patcher.exe .
go test ./...
```

build with Go 1.23 or newer.

> ##### p.s. I use ai, but it's still handled with extreme care because I know what I'm doing >///<
