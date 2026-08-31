> [!IMPORTANT]
> 中文文档可能落后于英文文档，如果有问题请先查看英文文档。
> 请使用英文发起新的 Issue, 以便于追踪和搜索

# tdl

<img align="right" src="docs/assets/img/logo.png" height="280" alt="">

> 📥 Telegram Downloader, but more than a downloader

<a href="README.md">English</a> | 简体中文

<p>
<img src="https://img.shields.io/github/go-mod/go-version/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/license/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/actions/workflow/status/iyear/tdl/master.yml?branch=master&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/v/release/iyear/tdl?color=red&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/downloads/iyear/tdl/total?style=flat-square" alt="">
</p>

#### 特性：

- 单文件启动
- 低资源占用
- 吃满你的带宽
- 比官方客户端更快
- 支持从受保护的会话中下载文件
- 具有自动回退和消息路由的转发功能
- 支持上传文件至 Telegram
- 导出历史消息/成员/订阅者数据至 JSON 文件
- **WebUI**：在浏览器中管理下载/上传/转发任务，快速调整各项参数

## WebUI（网页界面）

本 fork 内置了 Web 界面，一条命令启动：

```shell
tdl webui                # http://127.0.0.1:8080 (仅本机, 无需认证)
tdl webui --host 0.0.0.0 --token my-secret-token   # 远程访问, 使用 Bearer Token 认证
```

功能：

- 创建下载 / 上传 / 转发任务，CLI 全部参数均可通过表单调整
- 实时任务列表：进度、速度、ETA 与实时日志（SSE 推送）
- Telegram 扫码登录、会话列表浏览与搜索
- 全局设置（namespace、proxy、threads、limit、pool、delay 等）
- 任务通过自执行 tdl 子进程逐个运行，行为与 CLI 完全一致

Docker 部署：

```shell
docker run -d --name tdl-webui \
  -p 8080:8080 \
  -v tdl-data:/root/.tdl \
  -v tdl-downloads:/downloads \
  -e TDL_WEBUI_TOKEN=change-me \
  tdl_webui:0.1.0
```

完整开发计划见 [docs/webui-plan.md](docs/webui-plan.md)。

## 预览

预览中的速度已经达到了代理的限制，同时**速度取决于你是否是付费用户**

![](docs/assets/img/preview.gif)

## 文档

请参考 [文档](https://docs.iyear.me/tdl/zh/).

## 赞助者

![](https://raw.githubusercontent.com/iyear/sponsor/master/sponsors.svg)

## 贡献者
<a href="https://github.com/iyear/tdl/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=iyear/tdl&max=750&columns=20" alt="contributors"/>
</a>

## 协议

AGPL-3.0 License
