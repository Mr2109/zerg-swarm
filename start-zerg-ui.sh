#!/bin/bash
# zerg-ui 启动器(无代理版)——2026-09-08
# 背景: UI(reqwest)会走系统/环境代理(Clash 7895)转发 127.0.0.1:8580 → 失败 → "主控离线"
# 修法: 剥掉全部代理 env + NO_PROXY 本机回环 → 直连主控
# 用法: ./start-zerg-ui.sh   (或双击——需先 chmod +x)
cd "$(dirname "$0")"
exec env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u http_proxy -u https_proxy -u all_proxy \
  NO_PROXY='127.0.0.1,localhost' no_proxy='127.0.0.1,localhost' ./bin/zerg-ui
