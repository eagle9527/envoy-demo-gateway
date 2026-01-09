# Envoy Gateway 验证示例应用

这个项目提供了一个基于 Gin 的 Go 后端应用，以及配套的 Kubernetes 和 Gateway API 配置，用于验证 Envoy Gateway 的核心功能。

## 功能验证点

*   **Ingress 路径重写 (Path Rewrite)**: 验证网关能否正确修改请求路径。
*   **跨域资源共享 (CORS)**: 验证跨域请求处理。
*   **自定义头信息 (Custom Headers)**: 验证请求头透传及网关注入头信息。
*   **路由匹配**: 基于 Header 或 Path 的路由规则。

## 目录结构

*   `main.go`: 后端应用源码，内置了用于验证的特定 API。
*   `Dockerfile`: 应用镜像构建文件。
*   `k8s-app.yaml`: 应用的 Kubernetes Deployment 和 Service 定义。
*   `route-rewrite.yaml`: 场景 1 - 路径重写测试用例。
*   `route-header.yaml`: 场景 2 - 自定义 Header 路由测试用例。
*   `route-default.yaml`: 场景 3 - 默认路由（不加 CORS）测试用例。
*   `route-default-cors.yaml`: 场景 4 - 默认路由（加 CORS）测试用例。

## 快速开始

### 1. 部署后端应用

首先构建镜像（如果你使用 Kind 或本地集群，确保镜像可被拉取）：

```bash
# 构建镜像
docker build -t demo-app:latest .

# 如果使用 Kind，加载镜像到集群
# kind load docker-image demo-app:latest -n kind1
```

部署应用到 Kubernetes：

```bash
kubectl apply -f k8s-app.yaml
```

### 2. 配置网关路由

应用 Gateway API 配置。请确保你已经安装了 Envoy Gateway，并且有一个名为 `eg` 的 Gateway 实例（或者修改 yaml 中的 `parentRefs`）。

```bash
# 应用所有测试用例
kubectl apply -f route-rewrite.yaml
kubectl apply -f route-header.yaml
kubectl apply -f route-default.yaml
kubectl apply -f route-default-cors.yaml
```

**注意**: 默认路由分成了两个 Host：`demo.example.com`（不加 CORS）和 `cors.demo.example.com`（加 CORS）。你需要确保测试时使用对应 Host，或者修改本地 `/etc/hosts` 指向网关 IP。

---

## 测试用例

假设你的 Envoy Gateway 入口 IP 为 `GATEWAY_IP` (例如 `LoadBalancer` 的 IP 或 `NodePort`)。

### 场景 1: 验证路径重写 (Path Rewrite)

*   **配置文件**: `route-rewrite.yaml`
*   **配置**: 路由规则将 `/gateway/prefix/api` 前缀重写为 `/api`。
*   **测试命令**:

```bash
curl -v -H "Host: demo.example.com" \
  http://<GATEWAY_IP>/gateway/prefix/api/rewrite-test
```

*   **预期结果**:
    *   状态码: `200 OK`
    *   响应 JSON 中 `"received_path"` 应为 `/api/rewrite-test` (前缀被成功去掉)。

### 场景 2: 验证自定义 Header 路由与透传

*   **配置文件**: `route-header.yaml`
*   **配置**: 只有包含 `X-Debug: true` 的请求且路径为 `/debug` 开头才会被路由到后端。
*   **测试命令**:

```bash
curl -v -H "Host: demo.example.com" \
  -H "X-Debug: true" \
  -H "X-Custom-Client: my-client" \
  http://<GATEWAY_IP>/debug/headers
```

*   **预期结果**:
    *   状态码: `200 OK`
    *   响应 JSON 中应包含你发送的 `"X-Custom-Client": "my-client"`。
    *   同时可以查看到 Envoy 自动注入的 `X-Forwarded-For` 等头信息。

### 场景 3: 默认路由不返回跨域头

*   **配置文件**: `route-default.yaml`
*   **配置**: 不配置入口层 CORS；响应里不应该出现任何 `Access-Control-*` 头。
*   **测试命令**:

```bash
curl -v -H "Host: demo.example.com" \
  -H "Origin: http://frontend.test.com" \
  http://<GATEWAY_IP>/api/users
```

*   **预期结果**:
    *   状态码: `200 OK`
    *   响应头里**不包含** `Access-Control-Allow-Origin` 等 CORS 相关 Header

### 场景 4: 默认路由允许跨域（入口层处理）

*   **配置文件**: `route-default-cors.yaml`
*   **配置**: CORS 在入口层通过 HTTPRoute 的 `CORS` filter 处理；后端应用不返回任何 `Access-Control-*` 头。
*   **测试命令**: 发送一个预检请求 (Preflight)。

```bash
curl -v -X OPTIONS \
  -H "Host: cors.demo.example.com" \
  -H "Origin: http://frontend.test.com" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: x-custom-client,x-debug,content-type" \
  http://<GATEWAY_IP>/api/users
```

*   **预期结果**:
    *   状态码: `204 No Content` 或 `200 OK`
    *   响应头包含:
        *   `Access-Control-Allow-Origin: http://frontend.test.com`
        *   `Access-Control-Allow-Methods: ...`

再验证一次“实际请求”的响应头是否也包含 `Access-Control-Allow-Origin`：

```bash
curl -v -H "Host: cors.demo.example.com" \
  -H "Origin: http://frontend.test.com" \
  http://<GATEWAY_IP>/api/users
```

## 查看日志

为了进一步验证，你可以查看后端 Pod 的日志，它会打印所有请求的详细信息：

```bash
# 获取 Pod 名称
kubectl get pods -l app=demo-app

# 查看日志
kubectl logs -f <pod-name>
```

日志示例:
```text
[Request] Host=demo.example.com Method=GET Path=/api/rewrite-test RemoteAddr=10.244.0.1:12345
[Gateway Header] X-Forwarded-Proto: http
[Gateway Header] X-Envoy-Internal: true
...
```
