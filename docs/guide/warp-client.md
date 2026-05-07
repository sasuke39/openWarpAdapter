# Warp Client

`open-warp` depends on a patched Warp client. The repository includes patches under `patches/`.

## What the patches do

The patched client:

- uses a local channel
- sends AI requests to `http://127.0.0.1:18888`
- skips Firebase authentication for local adapter requests
- recognizes CJK text as natural-language AI input
- adds a local adapter settings menu item
- skips onboarding for local adapter builds
- gracefully stops the adapter helper when the app exits

## Build from source

Prerequisites:

- Go 1.22 or newer
- Rust toolchain
- compatible Warp source tree

Build:

```bash
git clone https://github.com/sasuke39/open-warp.git
cd open-warp
WARP_SRC=/path/to/warp-source sh ./build_and_bundle.sh
open ./WarpLocal.app
```

The resulting app bundle contains:

```text
WarpLocal.app/
└── Contents/
    ├── MacOS/warp
    ├── Helpers/warp-local-adapter
    └── Resources/
        ├── config.example.yaml
        └── iconfile.icns
```

See `WARP_CLIENT.md` in the repository for patch-level details.
