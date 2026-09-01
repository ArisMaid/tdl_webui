# tdl

<img align="right" src="docs/assets/img/logo.png" height="280" alt="">

> 📥 Telegram Downloader, but more than a downloader

English | <a href="README_zh.md">简体中文</a>

<p>
<img src="https://img.shields.io/github/go-mod/go-version/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/license/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/actions/workflow/status/iyear/tdl/master.yml?branch=master&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/v/release/iyear/tdl?color=red&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/downloads/iyear/tdl/total?style=flat-square" alt="">
</p>

#### Features:
- Single file start-up
- Low resource usage
- Take up all your bandwidth
- Faster than official clients
- Download files from (protected) chats
- Forward messages with automatic fallback and message routing
- Upload files to Telegram
- Export messages/members/subscribers to JSON
- **WebUI**: manage downloads/uploads/forwards and adjust parameters from your browser

## WebUI (browser interface)

This fork ships a built-in web interface. Start it with:

```shell
tdl webui                # http://127.0.0.1:8080 (localhost, no auth)
tdl webui --host 0.0.0.0 --token my-secret-token   # remote access with bearer token
```

Features:

- Download / upload / forward task creation with all CLI parameters exposed as form fields
- Real-time task list with progress, speed, ETA and live logs (Server-Sent Events)
- QR-code login to Telegram and chat list browsing
- Global settings (namespace, proxy, threads, limit, pool, delay, ...)
- Tasks run through the `tdl` binary itself as child processes, one at a time, so behavior is identical to the CLI

Docker:

```shell
docker run -d --name tdl-webui \
  -p 8080:8080 \
  -v tdl-data:/root/.tdl \
  -v tdl-downloads:/downloads \
  -e TDL_WEBUI_TOKEN=change-me \
  tdl_webui:0.1.0
```

Prebuilt multi-arch images (linux/amd64, linux/arm64, linux/arm/v7) are published to GitHub Container Registry by GitHub Actions on every tag and `master` push:

```
docker login ghcr.io -u ArisMaid   # needs a personal access token with read:packages scope
docker pull ghcr.io/arismaid/tdl_webui:0.1.0
docker run -d --name tdl-webui -p 8080:8080 -e TDL_WEBUI_TOKEN=change-me ghcr.io/arismaid/tdl_webui:0.1.0
```

> The package is private by default. To pull without a token on other devices, make it public: repo page → Packages → tdl_webui → Package settings → Change visibility → Public.

See [docs/webui-plan.md](docs/webui-plan.md) for the full development plan.

## Preview

It reaches my proxy's speed limit, and the **speed depends on whether you are a premium**

![](docs/assets/img/preview.gif)

## Documentation

Please refer to the [documentation](https://docs.iyear.me/tdl/).

## Sponsors

![](https://raw.githubusercontent.com/iyear/sponsor/master/sponsors.svg)

## Contributors
<a href="https://github.com/iyear/tdl/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=iyear/tdl&max=750&columns=20" alt="contributors"/>
</a>

## LICENSE

AGPL-3.0 License
