基于 Go 实现的设施番茄熊蜂授粉 Web 项目，一款后端服务，实现授粉轮换、农药冻结恢复、覆盖核验与双人结案闭环。

# tomato-bumblebee-pollination-rotation

本 Git 项目来自模型完成任务后的 workspace，不包含嵌套 .git 记录或本地构建产物。

## 本地构建与测试

```bash
go mod download
go build ./...
go test ./...
./run_benzhi_smoke.sh
```

## Docker 构建与运行

```bash
./build_benzhi_docker.sh tomato-bumblebee-pollination-rotation linux/arm64
docker run --rm -it --platform linux/arm64 tomato-bumblebee-pollination-rotation:latest
./build_benzhi_docker.sh tomato-bumblebee-pollination-rotation linux/amd64
docker run --rm -it --platform linux/amd64 tomato-bumblebee-pollination-rotation:latest
```

构建脚本第二个参数为目标平台，必须分别完成 linux/arm64 和 linux/amd64 构建与容器验证；未提供时按照规范默认使用 linux/amd64。系统 backend-v2 模板通过 Go 原生交叉编译生成目标架构的 /usr/local/bin/benzhi-app，镜像默认直接运行该入口。
