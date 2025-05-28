#!/usr/bin/env bash

# 确保脚本抛出遇到的错误
set -e


# 如果发布到 https://<USERNAME>.github.io/<REPO>
# git push -f git@gitee.com:ling-cn/iot-zinx.git main
# git push -f git@github.com:taoyao-code/iot-zinx.git main

echo "发布到 Gitee "

git push -f git@gitee.com:ling-cn/iot-zinx.git main

# echo "发布到 GitHub "
# git push -f git@github.com:taoyao-code/iot-zinx.git main
echo "发布成功！"