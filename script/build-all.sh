#!/usr/bin/env bash
set -e

# 定义所有应用
APPS=("gateway" "device-simulator" "notification-stats")
OUT="bin"             # 输出目录
PLATFORMS=("linux/amd64" "linux/arm64" "darwin/arm64" "windows/amd64")

# 是否启用 CGO（0=纯Go，1=需要C依赖）
USE_CGO=${USE_CGO:-0}

echo "📦 Building apps: ${APPS[*]}"
echo "🏗️  Platforms: ${PLATFORMS[*]}"
echo "🔧 CGO Mode: ${USE_CGO}"

rm -rf "$OUT" && mkdir -p "$OUT"

# 为每个应用构建
for APP in "${APPS[@]}"; do
  echo -e "\n🚀 Building application: $APP"
  mkdir -p "$OUT/$APP"
  
  for p in "${PLATFORMS[@]}"; do
    GOOS=${p%/*}
    GOARCH=${p#*/}

    BIN="$OUT/$APP/$APP-$GOOS-$GOARCH"
    [[ $GOOS == "windows" ]] && BIN="$BIN.exe"

    echo -e "\n  ==> � Building $APP for $GOOS/$GOARCH ..."

    # 构建路径指向 cmd/$APP
    BUILD_PATH="./cmd/$APP"

    if [[ "$USE_CGO" == "1" ]]; then
      if [[ "$GOOS" == "linux" ]]; then
        echo "    🔗 Linux CGO enabled + musl static build"
        CC=musl-gcc \
        CGO_ENABLED=1 \
        GOOS=$GOOS GOARCH=$GOARCH \
        go build -ldflags="-linkmode external -extldflags -static -s -w" -o "$BIN" "$BUILD_PATH"

      elif [[ "$GOOS" == "windows" ]]; then
        echo "    🔗 Windows CGO static build (MinGW-w64 required)"
        CC=x86_64-w64-mingw32-gcc \
        CGO_ENABLED=1 \
        GOOS=$GOOS GOARCH=$GOARCH \
        go build -ldflags="-extldflags=-static -s -w" -o "$BIN" "$BUILD_PATH"

      else
        echo "    ⚠️  CGO cross-compile for $GOOS not supported, falling back to pure Go"
        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
        go build -ldflags="-s -w" -o "$BIN" "$BUILD_PATH"
      fi

    else
      echo "    ✅ Pure Go build (CGO disabled)"
      CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
      go build -ldflags="-s -w" -o "$BIN" "$BUILD_PATH"
    fi

    # ✅ 验证是否静态（仅 Linux / Windows）
    if [[ "$GOOS" == "linux" && -x "$BIN" ]]; then
      echo "    🔍 Checking binary type (ldd):"
      if command -v ldd >/dev/null; then
        ldd "$BIN" || echo "    ✅ Not a dynamic executable"
      fi
    fi

    if [[ "$GOOS" == "windows" && -x "$BIN" ]]; then
      echo "    🔍 Checking Windows deps (objdump):"
      if command -v x86_64-w64-mingw32-objdump >/dev/null; then
        x86_64-w64-mingw32-objdump -p "$BIN" | grep DLL || echo "    ✅ No extra DLL deps"
      else
        echo "    (no objdump, skip check)"
      fi
    fi

    echo "    ✅ $BIN built."
  done
  
  echo "  📁 $APP binaries completed in $OUT/$APP/"
done

echo -e "\n🎉 All applications built successfully!"
echo "📁 Binaries are organized in $OUT/{app_name}/"

