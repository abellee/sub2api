// Imported documentation data; API origins and environment variable names are adapted for TokenFree.
export const referencePages = [
  {
    "id": "image2",
    "label": "默认分组",
    "title": "默认分组 · GPT Image",
    "lead": "默认分组的基础1K与Adobe兼容型号，分别列出模型、画质、比例尺寸、参考图、局部编辑和任务处理。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "文生图",
            "POST /v1/images/generations"
          ],
          [
            "图生图 / 编辑",
            "POST /v1/images/edits"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "分组",
            "default（默认分组）"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"YOUR_API_KEY\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"YOUR_API_KEY\""
      },
      {
        "type": "paragraph",
        "value": "使用获准访问本分组的Key。同名模型可以出现在不同分组中，仅修改model不会切换Key的分组选路。普通Key使用自身分组；生图和视频Key按获授权的分组顺序匹配，需要固定某条线路时使用对应分组的普通Key。"
      },
      {
        "type": "heading",
        "value": "选择模型"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "尺寸档位",
          "quality",
          "background",
          "mask局部编辑",
          "线路"
        ],
        "rows": [
          [
            "gpt-image-2",
            "仅1K",
            "auto / low / medium / high",
            "不提供，省略",
            "不开放",
            "基础1K"
          ],
          [
            "gpt-image-2.5-flare",
            "仅1K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "不开放",
            "基础1K"
          ],
          [
            "gpt-image-2.5-sunburst",
            "仅1K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "不开放",
            "基础1K"
          ],
          [
            "gpt-image-2-4k",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-flare-4k",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-flare-满血",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-sunburst-4k",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-sunburst-满血",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ]
        ]
      },
      {
        "type": "note",
        "value": "不带后缀的gpt-image-2、gpt-image-2.5-flare、gpt-image-2.5-sunburst保持基础1K能力。-满血、-4k后缀按Adobe线路使用。"
      },
      {
        "type": "heading",
        "value": "文生图参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求与说明"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "完整填写本页列出的模型名，保留AZ-、官-或规格后缀"
          ],
          [
            "prompt",
            "string，必填",
            "描述主体、构图、风格及画中文字；Adobe型号非空且不超过32768字节（不是32768个汉字）"
          ],
          [
            "size",
            "string，建议显式填写",
            "具体像素宽x高，按本页型号能力和尺寸表选择"
          ],
          [
            "quality",
            "string，可选",
            "按型号表选择；quality改变细节偏好，不改变size的格式"
          ],
          [
            "n",
            "integer，建议1",
            "Adobe支持1–4，省略为1；基础和其他线路建议每次1张。输出数量与参考图张数不同"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时使用auto、opaque或transparent；Adobe的gpt-image-2及gpt-image-2-4k不发送此字段"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json，控制返回结果，与输入参考图URL无关"
          ],
          [
            "output_format",
            "string，可选",
            "Adobe兼容输入接受png、jpeg、webp；保存时以实际Content-Type为准，不能保证此参数使上游按指定格式编码"
          ],
          [
            "output_compression",
            "integer，可选",
            "Adobe线路不提供独立压缩控制，省略；其他线路JPEG/WebP可选0–100，PNG省略"
          ],
          [
            "moderation",
            "string，可选",
            "Adobe未提供独立审核级别控制，Adobe型号省略；基础1K型号仍可使用auto或low"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Adobe的画质与透明背景"
      },
      {
        "type": "table",
        "headers": [
          "模型",
          "画质映射",
          "透明背景"
        ],
        "rows": [
          [
            "GPT Image 2（含默认分组gpt-image-2-4k）",
            "low→1；medium/auto→3；high/max→5；xhigh兼容映射为max",
            "不支持background和transparent_background，请省略"
          ],
          [
            "GPT Image 2.5 Flare / Sunburst的Adobe型号",
            "low→1；medium/auto→3；high→5；max/xhigh→7",
            "支持auto、opaque、transparent"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "画质映射是Adobe线路的细节等级。不要把官key或AZ的扩展品质行为套到Adobe；也不要把透明PNG遮罩与生成透明背景混为一谈。Image 2可以使用编辑遮罩，但不提供生成透明背景参数。"
      },
      {
        "type": "heading",
        "value": "比例、尺寸与像素限制"
      },
      {
        "type": "paragraph",
        "value": "size填写具体像素宽x高，不填写1K、4K或16:9。不带后缀的三个基础型号只使用1K列；本页Adobe后缀型号可使用1K、2K、4K。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size",
          "2K：size",
          "4K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024",
            "2048x2048",
            "2880x2880"
          ],
          [
            "3:2",
            "1536x1024",
            "2160x1440",
            "3456x2304"
          ],
          [
            "2:3",
            "1024x1536",
            "1440x2160",
            "2304x3456"
          ],
          [
            "16:9",
            "1280x720",
            "2560x1440",
            "3840x2160"
          ],
          [
            "9:16",
            "720x1280",
            "1440x2560",
            "2160x3840"
          ],
          [
            "4:3",
            "1024x768",
            "2048x1536",
            "3200x2400"
          ],
          [
            "3:4",
            "768x1024",
            "1536x2048",
            "2400x3200"
          ],
          [
            "21:9",
            "1280x544",
            "2560x1088",
            "3840x1600"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "约束",
          "要求"
        ],
        "rows": [
          [
            "宽高对齐",
            "宽和高均为16的倍数"
          ],
          [
            "最长边",
            "不超过3840px"
          ],
          [
            "长宽比",
            "最长边 / 最短边不超过3"
          ],
          [
            "总像素",
            "655360至8294400像素；同时遵守型号的档位限制"
          ],
          [
            "4K方图",
            "2880x2880，不是4096x4096"
          ],
          [
            "自定义尺寸",
            "同时满足对齐、最长边、长宽比和总像素要求；读取实际结果确认最终尺寸"
          ]
        ]
      },
      {
        "type": "note",
        "value": "Adobe高分辨率需要明确画幅，推荐直接传size，例如2048x2048或3840x2160。不要在2K、4K请求里使用auto来代替具体尺寸。"
      },
      {
        "type": "heading",
        "value": "文生图完整请求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"size\": \"1280x720\",\n  \"quality\": \"high\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "POST /v1/images/edits可使用本地文件或公开图片URL。单图JSON用image字符串；多图字段按下表的线路格式填写。不要把本机路径或网页预览地址当成图片URL。"
      },
      {
        "type": "table",
        "headers": [
          "输入方式",
          "Content-Type",
          "字段和限制"
        ],
        "rows": [
          [
            "本地文件",
            "multipart/form-data",
            "单图image；多图重复image[]，PNG、JPEG或WebP，最多16张参考图"
          ],
          [
            "单张URL",
            "application/json",
            "image为一张公开图片URL；与images二选一"
          ],
          [
            "多张URL",
            "application/json",
            "基础1K使用images字符串数组；Adobe后缀型号使用images对象数组，每项含image_url"
          ],
          [
            "mask",
            "multipart文件，可选",
            "仅本页Adobe后缀型号支持；基础1K不开放"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "编辑字段",
          "类型 / 要求",
          "填写方式"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "使用本页型号表中的完整名称，文生图和编辑的型号相同"
          ],
          [
            "prompt",
            "string，必填",
            "写清保留、替换或修改的内容，多图按顺序说明各图用途"
          ],
          [
            "size",
            "string，建议填写",
            "使用具体像素，遵守本页型号档位和尺寸限制"
          ],
          [
            "quality",
            "string，可选",
            "Adobe、AZ和官key按型号表选择；基础1K的整图编辑请求省略"
          ],
          [
            "n",
            "integer，建议1",
            "输出数量，不是参考图张数；Adobe支持1–4"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时传入；Adobe Image 2不发送"
          ],
          [
            "moderation",
            "string，可选",
            "基础1K、AZ和官key可选auto或low；Adobe不提供独立控制，省略"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json；与参考图片的输入方式无关"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；实际结果按Content-Type保存"
          ],
          [
            "mask",
            "文件，可选",
            "仅支持局部编辑的型号使用；透明区域表示修改范围"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "单张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"1280x720\",\n  \"n\": 1,\n  \"image\": \"https://example.com/product.png\"\n}'"
      },
      {
        "type": "heading",
        "value": "多张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"1280x720\",\n  \"n\": 1,\n  \"images\": [\n    \"https://example.com/product.png\",\n    \"https://example.com/style.jpg\"\n  ]\n}'"
      },
      {
        "type": "paragraph",
        "value": "默认分组的满血、4k后缀型号使用Adobe多图格式："
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare-满血\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"images\": [\n    {\n      \"image_url\": \"https://example.com/product.png\"\n    },\n    {\n      \"image_url\": \"https://example.com/style.jpg\"\n    }\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "本地图片上传"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=gpt-image-2.5-flare' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.jpg' \\\n  -F 'size=1280x720' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "multipart上传由HTTP客户端自动生成boundary，不手动填写Content-Type。URL必须直接返回图片文件且在排队、生成期间保持有效；不要使用image=@https://...。"
      },
      {
        "type": "heading",
        "value": "遮罩局部编辑"
      },
      {
        "type": "paragraph",
        "value": "mask与生成透明背景是不同功能。Adobe参考图与遮罩合计受64MiB请求限制；遮罩必须是带透明通道的PNG或WebP，宽高与第一张源图相同。本页基础1K型号不使用mask。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=gpt-image-2.5-flare-满血' \\\n  -F 'prompt=只替换遮罩透明区域中的背景，保留人物与衣服' \\\n  -F 'image=@original.png' \\\n  -F 'mask=@mask.png' \\\n  -F 'size=1024x1024' \\\n  -F 'quality=high' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "Adobe参考图和遮罩仅使用PNG、JPEG或WebP，总请求体不超过64MiB；Base64会增大请求体积。上传本地文件时优先multipart，避免多张图片编码后超限。"
      },
      {
        "type": "heading",
        "value": "透明背景"
      },
      {
        "type": "paragraph",
        "value": "Adobe仅GPT Image 2.5 Flare和Sunburst提供背景透明度参数；GPT Image 2及gpt-image-2-4k不支持，必须省略background和transparent_background。基础1K型号也不发送background。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare-满血\",\n  \"prompt\": \"一个独立的橘子图标，透明背景，无投影\",\n  \"size\": \"1024x1024\",\n  \"quality\": \"high\",\n  \"background\": \"transparent\",\n  \"output_format\": \"png\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图片结果字段与格式"
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "类型",
          "处理方式"
        ],
        "rows": [
          [
            "data",
            "array",
            "逐项处理生成结果，不丢弃后续图片"
          ],
          [
            "data[].url",
            "string，可选",
            "及时下载保存，临时URL不是永久存储"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "Base64解码成图片文件，不把编码文本直接写入.png"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "修订提示词，缺少此字段不等于生成失败"
          ],
          [
            "size / quality / background",
            "可选",
            "返回时记录实际参数，不凭请求值推断图片最终宽高"
          ],
          [
            "Content-Type",
            "文件下载响应头",
            "按实际MIME确定格式，不只根据URL后缀判断"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "同步响应需要在data中找到url或b64_json才算拿到图片。异步任务的result保留相同结构，有media归档地址时直接鉴权下载，客户端不需要转换接口协议。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"gpt-image-2.5-flare\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: 'gpt-image-2.5-flare', prompt: '浅色背景上的一杯橘子汽水', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "模型不存在或无可用渠道",
            "核对Key所属分组、授权范围及完整模型名；AZ使用flare，不是flara"
          ],
          [
            "response_format被拒绝",
            "AZ与官key请求省略此字段，不改成b64_json后自动重新提交"
          ],
          [
            "透明背景参数被拒绝",
            "Adobe的GPT Image 2不提供此功能；仅在本页型号表标明支持时发送"
          ],
          [
            "上传成功但参考图未生效",
            "核对image、images的线路格式及数组顺序，在prompt里说明每张参考图用途"
          ],
          [
            "连接超时或客户端关闭",
            "先查任务日志或已保存的任务编号，不自动重新创建收费任务"
          ],
          [
            "查询成功但生成失败",
            "HTTP 200只代表查询成功，仍需检查status、error和result_expired"
          ],
          [
            "价格不同",
            "同名模型在不同分组中可能使用不同计费规则和倍率，以当前Key报价和实际账单为准"
          ]
        ]
      }
    ]
  },
  {
    "id": "banana",
    "label": "香蕉",
    "title": "Nano Banana 2 / 2.1 / Pro",
    "lead": "三款香蕉型号的Gemini原生接入、比例与尺寸、URL或Base64参考图、图片编辑、异步查询及图片保存。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "值"
        ],
        "rows": [
          [
            "服务地址",
            "https://please.dontblockme.men"
          ],
          [
            "创建",
            "POST /v1beta/models/nano-banana-2:generateContent"
          ],
          [
            "鉴权",
            "Authorization: Bearer <API_KEY>"
          ],
          [
            "推荐模式",
            "Prefer: respond-async：提交 → 保存task_id → 查询poll_url → succeeded → 下载"
          ],
          [
            "同步模式",
            "省略异步开关，等待原生生成结果；长时间生成优先使用异步"
          ],
          [
            "任务查询",
            "GET /v1/tasks/{task_id}，使用创建账户的有效Key"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"<API_KEY>\""
      },
      {
        "type": "note",
        "value": "cURL示例采用Bash语法；PowerShell请用curl.exe并调整变量与换行。Key仅保存在本机环境变量或后端，不嵌入公开网页。尖括号内容需要替换；示例任务编号和时间用于说明返回结构。"
      },
      {
        "type": "heading",
        "value": "模型与接口"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "imageSize可用值",
          "比例数量",
          "型号差异",
          "参考图上限"
        ],
        "rows": [
          [
            "nano-banana-2",
            "512、1K、2K、4K",
            "14种",
            "支持512档位",
            "最多14张"
          ],
          [
            "nano-banana-2.1",
            "1K、2K、4K",
            "14种",
            "不支持512档位",
            "最多14张"
          ],
          [
            "nano-banana-pro",
            "1K、2K、4K",
            "10种",
            "不支持1:4、4:1、1:8、8:1",
            "最多14张"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "模型",
          "文生图 / 图生图接口"
        ],
        "rows": [
          [
            "nano-banana-2",
            "POST /v1beta/models/nano-banana-2:generateContent"
          ],
          [
            "nano-banana-2.1",
            "POST /v1beta/models/nano-banana-2.1:generateContent"
          ],
          [
            "nano-banana-pro",
            "POST /v1beta/models/nano-banana-pro:generateContent"
          ]
        ]
      },
      {
        "type": "note",
        "value": "香蕉三款型号统一使用Gemini原生接口，model写在URL里，不在JSON顶层添加model。文生图和图生图只相差parts中是否有参考图，不需要转换成Images或Chat格式。"
      },
      {
        "type": "heading",
        "value": "文生图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body 'https://please.dontblockme.men/v1beta/models/nano-banana-2:generateContent' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"contents\": [\n    {\n      \"role\": \"user\",\n      \"parts\": [\n        {\n          \"text\": \"画一只抱着橘子的小猫，柔和水彩风，浅色背景\"\n        }\n      ]\n    }\n  ],\n  \"generationConfig\": {\n    \"responseModalities\": [\n      \"TEXT\",\n      \"IMAGE\"\n    ],\n    \"imageConfig\": {\n      \"aspectRatio\": \"1:1\",\n      \"imageSize\": \"1K\"\n    }\n  }\n}'"
      },
      {
        "type": "heading",
        "value": "参数说明"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求 / 默认建议",
          "说明"
        ],
        "rows": [
          [
            "contents",
            "array",
            "必填",
            "单次生成传一条user消息，不用Images的prompt字段替代"
          ],
          [
            "contents[].role",
            "string",
            "建议user",
            "当前用户的生成要求"
          ],
          [
            "contents[].parts",
            "array",
            "必填",
            "至少一个text提示词；参考图使用inlineData或fileData，每张图单独一个part"
          ],
          [
            "contents[].parts[].text",
            "string",
            "必填",
            "多图时按排列顺序说明参考图用途"
          ],
          [
            "contents[].parts[].inlineData.mimeType",
            "string",
            "Base64传图时必填",
            "image/png、image/jpeg、image/webp，与文件内容一致"
          ],
          [
            "contents[].parts[].inlineData.data",
            "string",
            "Base64传图时必填",
            "纯Base64，不带data:image/png;base64,前缀，不填写URL或本机路径"
          ],
          [
            "contents[].parts[].fileData.mimeType",
            "string",
            "URL传图时填写",
            "image/png、image/jpeg、image/webp，与链接返回的图片内容一致"
          ],
          [
            "contents[].parts[].fileData.fileUri",
            "string",
            "URL传图时必填",
            "可访问的图片HTTPS直链；无需放入inlineData.data或在客户端转为Base64"
          ],
          [
            "generationConfig.responseModalities",
            "string[]",
            "建议[\"IMAGE\"]",
            "如需同时读取文字可用[\"TEXT\",\"IMAGE\"]"
          ],
          [
            "generationConfig.imageConfig.aspectRatio",
            "string",
            "建议1:1",
            "枚举值见下表，不使用1376x768等像素字符串"
          ],
          [
            "generationConfig.imageConfig.imageSize",
            "string",
            "建议1K",
            "1K、2K、4K区分大小写；仅nano-banana-2支持字符串\"512\"，不是512px或数字512"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "quality、background、output_format、n、size不是这份原生请求的控制字段。需要多张独立结果时创建多个请求并分别保存任务编号，不靠添加n保证批量出图。"
      },
      {
        "type": "heading",
        "value": "香蕉比例与输出尺寸"
      },
      {
        "type": "paragraph",
        "value": "请求只发送aspectRatio与imageSize，下表宽高用于说明输出尺寸，不作为size字段发送。前10种比例三款型号均支持；后4种超宽/超长比例仅用于nano-banana-2和nano-banana-2.1。512列仅用于nano-banana-2。"
      },
      {
        "type": "table",
        "headers": [
          "aspectRatio",
          "512：仅Banana 2",
          "1K",
          "2K",
          "4K"
        ],
        "rows": [
          [
            "1:1",
            "512x512",
            "1024x1024",
            "2048x2048",
            "4096x4096"
          ],
          [
            "3:2",
            "632x424",
            "1264x848",
            "2528x1696",
            "5056x3392"
          ],
          [
            "2:3",
            "424x632",
            "848x1264",
            "1696x2528",
            "3392x5056"
          ],
          [
            "16:9",
            "688x384",
            "1376x768",
            "2752x1536",
            "5504x3072"
          ],
          [
            "9:16",
            "384x688",
            "768x1376",
            "1536x2752",
            "3072x5504"
          ],
          [
            "4:3",
            "600x448",
            "1200x896",
            "2400x1792",
            "4800x3584"
          ],
          [
            "3:4",
            "448x600",
            "896x1200",
            "1792x2400",
            "3584x4800"
          ],
          [
            "4:5",
            "464x576",
            "928x1152",
            "1856x2304",
            "3712x4608"
          ],
          [
            "5:4",
            "576x464",
            "1152x928",
            "2304x1856",
            "4608x3712"
          ],
          [
            "21:9",
            "792x168",
            "1584x672",
            "3168x1344",
            "6336x2688"
          ],
          [
            "1:4",
            "256x1024",
            "512x2048",
            "1024x4096",
            "2048x8192"
          ],
          [
            "4:1",
            "1024x256",
            "2048x512",
            "4096x1024",
            "8192x2048"
          ],
          [
            "1:8",
            "192x1536",
            "384x3072",
            "768x6144",
            "1536x12288"
          ],
          [
            "8:1",
            "1536x192",
            "3072x384",
            "6144x768",
            "12288x1536"
          ]
        ]
      },
      {
        "type": "note",
        "value": "Pro的1K、2K、4K分别对应1024、2048、4096像素方图；2.1不支持512档位。21:9的512尺寸单独列出，不从1K等比推算；保存时读取实际图片宽高。"
      },
      {
        "type": "heading",
        "value": "Nano Banana 2.1完整请求"
      },
      {
        "type": "paragraph",
        "value": "下面是新增型号的16:9、2K创建请求。添加Prefer请求头后返回网关异步任务，随后按poll_url查询。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1beta/models/nano-banana-2.1:generateContent' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\"contents\":[{\"role\":\"user\",\"parts\":[{\"text\":\"花店门口的清晨插画，柔和日光，横版构图\"}]}],\"generationConfig\":{\"responseModalities\":[\"IMAGE\"],\"imageConfig\":{\"aspectRatio\":\"16:9\",\"imageSize\":\"2K\"}}}'"
      },
      {
        "type": "heading",
        "value": "512档位：仅Nano Banana 2"
      },
      {
        "type": "paragraph",
        "value": "以下JSON只用于nano-banana-2:generateContent，不发送给2.1或Pro。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"contents\": [\n    {\n      \"role\": \"user\",\n      \"parts\": [\n        {\n          \"text\": \"一个橘子图标，白色背景\"\n        }\n      ]\n    }\n  ],\n  \"generationConfig\": {\n    \"responseModalities\": [\n      \"IMAGE\"\n    ],\n    \"imageConfig\": {\n      \"aspectRatio\": \"1:1\",\n      \"imageSize\": \"512\"\n    }\n  }\n}"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "文字与参考图放在同一个contents[0].parts数组里，第一项写生成或编辑要求。参考图支持本地图片Base64（inlineData）和图片URL（fileData）；每张图单独一个part，同一个part不同时放inlineData与fileData。"
      },
      {
        "type": "table",
        "headers": [
          "输入来源",
          "字段",
          "内容"
        ],
        "rows": [
          [
            "本地图片或已有Base64",
            "inlineData.mimeType + inlineData.data",
            "实际MIME和纯Base64，不含Data URL前缀"
          ],
          [
            "远程图片URL",
            "fileData.mimeType + fileData.fileUri",
            "实际MIME和图片HTTPS直链，无需客户端下载、编码"
          ],
          [
            "多张参考图",
            "多个inlineData / fileData part",
            "可按各图来源分别选择；三款型号合计最多14张"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "本地参考图：inlineData"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"contents\": [\n    {\n      \"role\": \"user\",\n      \"parts\": [\n        {\n          \"text\": \"保留第一张图的人物，使用第二张图的服装，背景改成纯白\"\n        },\n        {\n          \"inlineData\": {\n            \"mimeType\": \"image/png\",\n            \"data\": \"<FIRST_IMAGE_BASE64>\"\n          }\n        },\n        {\n          \"inlineData\": {\n            \"mimeType\": \"image/jpeg\",\n            \"data\": \"<SECOND_IMAGE_BASE64>\"\n          }\n        }\n      ]\n    }\n  ],\n  \"generationConfig\": {\n    \"responseModalities\": [\n      \"IMAGE\"\n    ],\n    \"imageConfig\": {\n      \"aspectRatio\": \"3:4\",\n      \"imageSize\": \"2K\"\n    }\n  }\n}"
      },
      {
        "type": "heading",
        "value": "URL图生图：fileData"
      },
      {
        "type": "paragraph",
        "value": "使用fileData.fileUri填写图片URL，不把URL写入inlineData.data，也不在请求顶层添加images。下面将远程图片作为参考，三款型号均使用同一请求结构，切换时替换URL中的完整模型名称。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1beta/models/nano-banana-2.1:generateContent' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"contents\": [\n    {\n      \"role\": \"user\",\n      \"parts\": [\n        {\n          \"text\": \"参考图片中的商品外观，生成一张简洁的商业海报，背景为浅蓝色\"\n        },\n        {\n          \"fileData\": {\n            \"mimeType\": \"image/png\",\n            \"fileUri\": \"https://example.com/product.png\"\n          }\n        }\n      ]\n    }\n  ],\n  \"generationConfig\": {\n    \"responseModalities\": [\n      \"IMAGE\"\n    ],\n    \"imageConfig\": {\n      \"aspectRatio\": \"3:4\",\n      \"imageSize\": \"2K\"\n    }\n  }\n}'"
      },
      {
        "type": "table",
        "headers": [
          "检查项",
          "要求"
        ],
        "rows": [
          [
            "格式",
            "PNG、JPEG或WebP，mimeType与实际内容一致"
          ],
          [
            "Base64",
            "不含Data URL前缀，不把URL或文件名放入data"
          ],
          [
            "URL",
            "必须直接返回图片，不能是网页、网盘预览页、本机地址或需要Cookie登录的链接；签名链接需在排队与生成期间有效"
          ],
          [
            "多图顺序",
            "提示词中的第一张、第二张对应所有图片part的排列顺序，与使用inlineData还是fileData无关"
          ],
          [
            "数量",
            "三款型号总输入最多14张参考图，不将参考图数量等同于输出数量"
          ],
          [
            "体积",
            "URL减少请求体积，但不绕过图片数量及文件大小限制；Base64约比原文件大三分之一，大图先压缩，413后不要原样重复提交"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "图片编辑：换背景、局部修改与多图组合"
      },
      {
        "type": "paragraph",
        "value": "nano-banana-2、nano-banana-2.1和nano-banana-pro均支持基于参考图的图片编辑，仍调用本页generateContent接口。把原图作为参考图传入，text明确写出要修改的内容和必须保留的部分；URL与Base64两种输入都可用于编辑。"
      },
      {
        "type": "table",
        "headers": [
          "编辑目标",
          "提示词写法"
        ],
        "rows": [
          [
            "更换背景",
            "保留商品的形状、颜色和标签，仅将背景替换为白色摄影棚"
          ],
          [
            "局部修改",
            "仅将人物手中的杯子改为蓝色，保留人物面容、姿态、衣服与其他区域"
          ],
          [
            "修改文字",
            "将招牌上的文字替换为“橘子汽水”，保留招牌材质、透视与环境"
          ],
          [
            "多图组合",
            "保留第一张图的人物外观，换上第二张图的服装，不改变人物姿态"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1beta/models/nano-banana-pro:generateContent' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"contents\": [\n    {\n      \"role\": \"user\",\n      \"parts\": [\n        {\n          \"text\": \"编辑第一张图：保持人物面容、姿态与背景不变，只将衣服换成第二张图的款式与颜色\"\n        },\n        {\n          \"fileData\": {\n            \"mimeType\": \"image/png\",\n            \"fileUri\": \"https://example.com/person.png\"\n          }\n        },\n        {\n          \"fileData\": {\n            \"mimeType\": \"image/jpeg\",\n            \"fileUri\": \"https://example.com/clothes.jpg\"\n          }\n        }\n      ]\n    }\n  ],\n  \"generationConfig\": {\n    \"responseModalities\": [\n      \"IMAGE\"\n    ],\n    \"imageConfig\": {\n      \"aspectRatio\": \"3:4\",\n      \"imageSize\": \"2K\"\n    }\n  }\n}'"
      },
      {
        "type": "note",
        "value": "这里的局部修改通过文字指令表达，不使用GPT Images接口的mask字段，也不承诺未修改区域逐像素一致。需要继续编辑时，把上一次结果作为新参考图再次提交；异步任务编号用于查询结果，不是编辑会话编号。"
      },
      {
        "type": "heading",
        "value": "参考图组合与主体一致性"
      },
      {
        "type": "table",
        "headers": [
          "型号",
          "物体参考",
          "人物一致性参考",
          "独立风格参考",
          "总上限"
        ],
        "rows": [
          [
            "nano-banana-2",
            "最多10张",
            "最多4张",
            "使用提示词描述风格",
            "14张"
          ],
          [
            "nano-banana-2.1",
            "最多10张",
            "最多4张",
            "使用提示词描述风格",
            "14张"
          ],
          [
            "nano-banana-pro",
            "最多6张",
            "最多5张",
            "最多3张",
            "14张"
          ]
        ]
      },
      {
        "type": "note",
        "value": "上表描述参考主体的高保真组合能力，不表示一定逐像素保留原图。将人物、商品与风格用途写进text，所有inlineData与fileData图片合计不超过14项；不要通过重复同一图片填满数量。"
      },
      {
        "type": "heading",
        "value": "同步响应与结果保存"
      },
      {
        "type": "paragraph",
        "value": "省略Prefer请求头和async查询参数时，成功响应直接返回以下Gemini原生结构。HTTP 2xx不保证一定包含可用图片，应校验实际图片项。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"candidates\": [\n    {\n      \"content\": {\n        \"parts\": [\n          {\n            \"inlineData\": {\n              \"mimeType\": \"image/png\",\n              \"data\": \"<BASE64_IMAGE_DATA>\"\n            }\n          }\n        ]\n      }\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "读取方式"
        ],
        "rows": [
          [
            "candidates",
            "array",
            "遍历所有候选项，不固定只取第一个"
          ],
          [
            "content.parts",
            "array",
            "同一响应可能同时包含文字和图片"
          ],
          [
            "inlineData.mimeType",
            "string",
            "按实际MIME保存扩展名"
          ],
          [
            "inlineData.data",
            "string",
            "纯Base64，客户端解码后写入文件"
          ],
          [
            "finishReason / promptFeedback",
            "可选",
            "无图片时检查结束原因或内容拦截提示"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "遍历candidates[].content.parts[]，对包含inlineData.data的项解码。文本响应、空candidates或仅有promptFeedback都不应标记为出图成功。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"gemini_image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"nano-banana-2\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1beta/models/nano-banana-2:generateContent\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"candidates\": [\n      {\n        \"content\": {\n          \"parts\": [\n            {\n              \"inlineData\": {\n                \"mimeType\": \"image/png\",\n                \"data\": \"<BASE64_IMAGE_DATA>\"\n              }\n            }\n          ]\n        }\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "paragraph",
        "value": "完整示例默认nano-banana-2。调用新型号可设置TOKENFREE_IMAGE_MODEL=nano-banana-2.1，调用Pro可设置为nano-banana-pro；三款型号都支持示例的1:1、1K配置。恢复任务时只读取已保存编号，不再次生成。"
      },
      {
        "type": "paragraph",
        "value": "参考图示例需要同目录的reference.png。纯文生图时删除读取reference.png及对应inlineData项即可；恢复任务时不会重新读取参考图。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n// 可切换三款公开型号；恢复已有任务时不重新生成。\nconst model = process.env.TOKENFREE_IMAGE_MODEL || 'nano-banana-2';\nif (!['nano-banana-2', 'nano-banana-2.1', 'nano-banana-pro'].includes(model)) {\n  throw new Error('请填写文档中的完整香蕉模型名称');\n}\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const image = await readFile('reference.png');\n  const response = await fetch(ORIGIN + '/v1beta/models/' + model + ':generateContent', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({\n      contents: [{ role: 'user', parts: [\n        { text: '保持主体外观，替换为纯白背景' },\n        { inlineData: { mimeType: 'image/png', data: image.toString('base64') } }\n      ] }],\n      generationConfig: { responseModalities: ['IMAGE'],\n        imageConfig: { aspectRatio: '1:1', imageSize: '1K' } }\n    }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "list",
        "items": [
          "HTTP 202只表示任务受理；先查询到succeeded，再读取图片，不把提交响应当作成品。",
          "参考图不生效：本地图片检查inlineData的纯Base64与mimeType，URL检查fileData.fileUri的可访问性；两种方式均按图片part顺序引用。",
          "没有图片：遍历candidates[].content.parts[]，检查inlineData以及finishReason、promptFeedback，纯文字不算出图成功。",
          "下载返回401：media[].url需要Bearer鉴权；临时展示链接也可能过期，不在URL中拼接自己的Key。",
          "排查时提供模型、task_id、请求时间、HTTP状态及去掉Key的参数，不发送完整Key。"
        ]
      },
      {
        "type": "links",
        "items": [
          {
            "id": "image2",
            "label": "GPT Image"
          },
          {
            "id": "seedream",
            "label": "Seedream"
          },
          {
            "id": "video",
            "label": "Wan 视频"
          },
          {
            "id": "sd-video",
            "label": "SD 视频"
          }
        ]
      }
    ]
  },
  {
    "id": "seedream",
    "label": "Seedream",
    "title": "Seedream 5 Pro",
    "lead": "1K、2K图片生成与最多10张参考图编辑，包含具体像素尺寸、异步任务与文件下载。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "值"
        ],
        "rows": [
          [
            "服务地址",
            "https://please.dontblockme.men"
          ],
          [
            "创建",
            "POST /v1/images/generations"
          ],
          [
            "鉴权",
            "Authorization: Bearer <API_KEY>"
          ],
          [
            "推荐模式",
            "Prefer: respond-async：提交 → 保存task_id → 查询poll_url → succeeded → 下载"
          ],
          [
            "同步模式",
            "省略异步开关，等待原生生成结果；长时间生成优先使用异步"
          ],
          [
            "任务查询",
            "GET /v1/tasks/{task_id}，使用创建账户的有效Key"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"<API_KEY>\""
      },
      {
        "type": "note",
        "value": "cURL示例采用Bash语法；PowerShell请用curl.exe并调整变量与换行。Key仅保存在本机环境变量或后端，不嵌入公开网页。尖括号内容需要替换；示例任务编号和时间用于说明返回结构。"
      },
      {
        "type": "heading",
        "value": "模型选择"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "接口",
          "尺寸档位",
          "参考图上限"
        ],
        "rows": [
          [
            "seedream-5-pro",
            "Images文生图与图片编辑",
            "1K、2K",
            "最多10张"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "文生图使用JSON，图片编辑使用multipart/form-data；不需要封装香蕉的contents，也不需要在服务器增加图片格式转换服务。"
      },
      {
        "type": "heading",
        "value": "文生图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\"model\":\"seedream-5-pro\",\"prompt\":\"一幅花店门口的清晨插画，柔和日光\",\"size\":\"2560x1440\",\"response_format\":\"b64_json\",\"n\":1}'"
      },
      {
        "type": "heading",
        "value": "参考图编辑"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=seedream-5-pro' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的色调和构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.png' \\\n  -F 'size=2048x2048' \\\n  -F 'response_format=b64_json' \\\n  -F 'n=1'"
      },
      {
        "type": "heading",
        "value": "常用字段"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求 / 默认建议",
          "说明"
        ],
        "rows": [
          [
            "model",
            "string",
            "必填",
            "seedream-5-pro"
          ],
          [
            "prompt",
            "string",
            "必填",
            "生成内容；编辑时按顺序说明参考图用途与保留细节"
          ],
          [
            "size",
            "string",
            "建议1024x1024",
            "使用下表具体宽高，不发送1K、2K或16:9"
          ],
          [
            "image / image[]",
            "文件",
            "图生图必填",
            "单图用image，多图重复image[]；最多10张PNG/JPEG/WebP参考图"
          ],
          [
            "n",
            "integer",
            "建议1",
            "输出数量，多次生成分别保存任务编号，不等同于参考图数量"
          ],
          [
            "response_format",
            "string",
            "建议b64_json",
            "读取data[].b64_json或data[].url，以实际响应为准"
          ],
          [
            "quality",
            "string",
            "基础示例省略",
            "不用GPT Image的xhigh/max，也不靠quality选择分辨率"
          ],
          [
            "background / mask",
            "扩展字段",
            "基础示例不传",
            "当前公开接入不承诺透明背景或局部遮罩能力；替换背景写入prompt"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Seedream比例与具体尺寸"
      },
      {
        "type": "paragraph",
        "value": "支持1K和2K，下表列出8种常用比例的size。不要使用4K尺寸，或直接复制香蕉的5504x3072等尺寸。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size",
          "2K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024",
            "2048x2048"
          ],
          [
            "3:2",
            "1536x1024",
            "2160x1440"
          ],
          [
            "2:3",
            "1024x1536",
            "1440x2160"
          ],
          [
            "16:9",
            "1280x720",
            "2560x1440"
          ],
          [
            "9:16",
            "720x1280",
            "1440x2560"
          ],
          [
            "4:3",
            "1024x768",
            "2048x1536"
          ],
          [
            "3:4",
            "768x1024",
            "1536x2048"
          ],
          [
            "21:9",
            "1280x544",
            "2560x1088"
          ]
        ]
      },
      {
        "type": "note",
        "value": "最多10张参考图，size使用英文x分隔。21:9会作像素对齐，输出以返回图片实际宽高为准。"
      },
      {
        "type": "heading",
        "value": "同步响应与结果保存"
      },
      {
        "type": "paragraph",
        "value": "省略Prefer请求头和async查询参数时，成功响应直接返回以下Images结构。HTTP 2xx不保证一定包含可用图片，应校验实际图片项。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"created\": 1790900000,\n  \"data\": [\n    {\n      \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "读取方式"
        ],
        "rows": [
          [
            "created",
            "integer",
            "Unix时间戳，单位秒"
          ],
          [
            "data",
            "array",
            "逐项读取图片结果"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "纯Base64，解码成图片文件"
          ],
          [
            "data[].url",
            "string，可选",
            "可能是HTTPS地址或Data URL，按实际格式处理"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "提示词修订；并非所有型号提供"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "b64_json使用Base64解码；data:image/...;base64,...先移除逗号前的前缀再解码。HTTPS结果地址通常有有效期，应及时保存；向第三方存储下载时不要携带平台Key。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"seedream-5-pro\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: 'seedream-5-pro', prompt: '浅色背景上的一杯橘子汽水', size: '2048x2048', response_format: 'b64_json', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "list",
        "items": [
          "HTTP 202只表示任务受理；先查询到succeeded，再读取图片，不把提交响应当作成品。",
          "参考图不生效：编辑接口使用image或image[]文件字段；multipart请求不要手动省略boundary。",
          "没有图片：检查result.data中的url或b64_json、error和result_expired，不只检查HTTP状态。",
          "下载返回401：media[].url需要Bearer鉴权；临时展示链接也可能过期，不在URL中拼接自己的Key。",
          "排查时提供模型、task_id、请求时间、HTTP状态及去掉Key的参数，不发送完整Key。"
        ]
      },
      {
        "type": "links",
        "items": [
          {
            "id": "image2",
            "label": "GPT Image"
          },
          {
            "id": "banana",
            "label": "Nano Banana"
          },
          {
            "id": "video",
            "label": "Wan 视频"
          },
          {
            "id": "sd-video",
            "label": "SD 视频"
          }
        ]
      }
    ]
  },
  {
    "id": "mj-v8.2",
    "label": "mj-v8.2",
    "title": "mj-v8.2",
    "lead": "29项请求参数、提示词长度、12种比例、URL与Data URL参考生图、质量与风格控制，以及异步任务示例。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "模型与接口"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "模型名称",
            "mj-v8.2"
          ],
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "创建任务",
            "POST /v1/midjourney/generations"
          ],
          [
            "查询任务",
            "GET /v1/tasks/{task_id}"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "请求格式",
            "Content-Type: application/json"
          ],
          [
            "普通生成输出",
            "一次任务4张单图"
          ],
          [
            "提示词长度",
            "prompt为1–1023个字符"
          ],
          [
            "参考图片",
            "单张image或1–5张images；URL或完整Data URL"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "`mj-v8.2`支持文生图和参考生图，可提供单张或多张参考图片。任务接口的外层字段兼容顶层和`data`包装，完成结果从`image_urls`读取。"
      },
      {
        "type": "heading",
        "value": "请求参数"
      },
      {
        "type": "table",
        "headers": [
          "参数",
          "类型",
          "必填",
          "取值或默认值",
          "用途"
        ],
        "rows": [
          [
            "model",
            "string",
            "是",
            "mj-v8.2",
            "固定填写公开模型名称`mj-v8.2`。"
          ],
          [
            "prompt",
            "string",
            "是",
            "1–1023个字符",
            "描述主体、场景、构图、画风、光线；参考图用途也写在这里。长度限制针对prompt字段，不是1023个单词或tokens。"
          ],
          [
            "size",
            "string",
            "否",
            "默认9:16；见下表",
            "填写画面比例，例如`16:9`，不是`1024x1024`这样的像素尺寸。"
          ],
          [
            "image",
            "string",
            "否",
            "公网图片URL或data:image/...",
            "单张参考图；有多张时使用`images`。"
          ],
          [
            "images",
            "string[]",
            "否",
            "1–5张",
            "每项为图片URL或data:image/...；数组顺序保留。单张和多张写法选一种即可。"
          ],
          [
            "raw",
            "boolean",
            "否",
            "默认false",
            "`true`启用Raw原始风格；传JSON布尔值，不要传字符串\"true\"。"
          ],
          [
            "negative_prompt",
            "string",
            "否",
            "按需填写",
            "描述希望排除的主体、元素或画面特征；不替代正向prompt。"
          ],
          [
            "seed",
            "integer",
            "否",
            "按需填写",
            "随机种子；未约定统一的默认值和数值范围，不保证不同参数组合复现相同画面。"
          ],
          [
            "stylize",
            "number",
            "否",
            "0–1000；默认100",
            "风格化强度，数值越高越偏向模型自身的美学表现。"
          ],
          [
            "chaos",
            "number",
            "否",
            "0–100；默认0",
            "控制结果之间的变化程度。"
          ],
          [
            "weird",
            "number",
            "否",
            "0–3000；默认0",
            "控制画面的非传统、怪异风格。"
          ],
          [
            "tile",
            "boolean",
            "否",
            "按需填写",
            "平铺纹理开关；使用JSON布尔值，不传字符串。"
          ],
          [
            "iw",
            "number",
            "否",
            "0–3",
            "普通参考图权重，与image或images配合，不代替参考图字段。"
          ],
          [
            "cref",
            "string",
            "否",
            "角色参考图URL",
            "用于角色参考；需要调整角色权重时配合cw。"
          ],
          [
            "sref",
            "string",
            "否",
            "风格参考图URL",
            "用于风格参考；需要调整风格权重时配合sw。"
          ],
          [
            "dref",
            "string",
            "否",
            "深度参考图URL",
            "用于深度参考；需要调整深度权重时配合dw。"
          ],
          [
            "cw",
            "number",
            "否",
            "0–100",
            "角色参考权重，与cref配合。"
          ],
          [
            "sw",
            "number",
            "否",
            "0–1000",
            "风格参考权重，与sref配合。"
          ],
          [
            "dw",
            "number",
            "否",
            "0–100",
            "深度参考权重，与dref配合。"
          ],
          [
            "quality",
            "number",
            "否",
            "0.25 / 0.5 / 1 / 2；默认1",
            "发送JSON数值，不是GPT Image的low、medium、high，也不是数字字符串。"
          ],
          [
            "hd",
            "boolean",
            "否",
            "默认false",
            "HD高清开关；不等同于size比例，也不约定固定输出像素。"
          ],
          [
            "n",
            "integer",
            "否",
            "固定1",
            "表示一次提交；普通生成一次返回4张单图，不要设置为4。"
          ],
          [
            "action",
            "string",
            "否",
            "见后续动作表",
            "对已有任务执行操作；普通文生图与参考生图不需要填写。"
          ],
          [
            "task_id",
            "string",
            "否",
            "父任务ID",
            "执行后续动作时指定原任务，不用于替换model或当前请求的鉴权。"
          ],
          [
            "index",
            "integer",
            "否",
            "按原任务操作指定",
            "选择原任务中的某张图片；未约定统一的索引范围，不自行假定0起始或1起始。"
          ],
          [
            "custom_id",
            "string",
            "否",
            "原任务提供的按钮ID",
            "执行对应按钮操作时填写，不自行拼接按钮编号。"
          ],
          [
            "direction",
            "string",
            "否",
            "按平移动作指定",
            "平移方向；示例值left，不将未列出的方向视为固定枚举。"
          ],
          [
            "zoom_ratio",
            "number",
            "否",
            "按扩图操作指定",
            "扩图倍数；未约定统一的范围和默认值。"
          ],
          [
            "dimensions",
            "string",
            "否",
            "按融图操作指定",
            "融图画幅；示例值SQUARE，不是像素尺寸，也不是size的替代字段。"
          ]
        ]
      },
      {
        "type": "note",
        "value": "prompt最长1023个字符。不要按tokens或单词数计算，也不要把参考图片的Base64拼进prompt；图片内容通过image或images单独提交。超长时先精简提示词，不依靠客户端静默截断。"
      },
      {
        "type": "heading",
        "value": "画面比例与Raw"
      },
      {
        "type": "table",
        "headers": [
          "size",
          "适用画幅"
        ],
        "rows": [
          [
            "auto",
            "由模型结合提示词或参考图判断"
          ],
          [
            "1:1",
            "正方形、头像、商品图"
          ],
          [
            "3:2",
            "横向摄影"
          ],
          [
            "2:3",
            "竖向摄影"
          ],
          [
            "4:3",
            "横向插画"
          ],
          [
            "3:4",
            "竖向插画"
          ],
          [
            "5:4",
            "横向展示"
          ],
          [
            "4:5",
            "竖向海报"
          ],
          [
            "16:9",
            "横向宽屏"
          ],
          [
            "9:16",
            "竖屏；默认比例"
          ],
          [
            "21:9",
            "超宽横图"
          ],
          [
            "9:21",
            "超长竖图"
          ]
        ]
      },
      {
        "type": "note",
        "value": "`size`填写比例，不提供固定输出像素保证；`n`固定1，普通生成返回4张单图。默认比例为9:16，推荐在请求中明确指定所需比例。"
      },
      {
        "type": "paragraph",
        "value": "Raw通过`raw: true`开启。优先使用size、raw及下表列出的独立参数，不在prompt中重复拼接相互冲突的--ar、--raw等指令。quality填写数值0.25、0.5、1或2，不使用其他图片模型的质量档位名称。"
      },
      {
        "type": "heading",
        "value": "文生图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/midjourney/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"一杯橘子汽水，玻璃杯上的水珠，清晨自然光，精致商业摄影\",\n  \"size\": \"1:1\",\n  \"raw\": false,\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "画质与风格控制"
      },
      {
        "type": "paragraph",
        "value": "以下是在普通生成请求中加入可选参数的写法。stylize、chaos、weird分别控制风格化、画面变化和怪异程度；negative_prompt描述不希望出现的内容。hd与raw为布尔值，quality及强度参数为数值。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/midjourney/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"negative_prompt\": \"模糊，多余文字\",\n  \"size\": \"4:5\",\n  \"raw\": false,\n  \"quality\": 1,\n  \"hd\": false,\n  \"stylize\": 100,\n  \"chaos\": 0,\n  \"weird\": 0,\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "参考生图（单张与多张参考图）"
      },
      {
        "type": "paragraph",
        "value": "提供参考图片并配合提示词生成新的图片。单张参考图使用`image`，多张参考图使用`images`；在提示词中说明希望参考的主体、配色、光线或构图。"
      },
      {
        "type": "paragraph",
        "value": "创建接口仍为POST /v1/midjourney/generations，使用application/json，不是multipart文件上传。图片URL必须直接返回公网可读的图片文件，不能是网页、本机路径或需要登录的地址。使用Base64时传完整的`data:image/png;base64,...`或对应图片类型的Data URL，不能只传裸Base64。多图按数组顺序说明各张图的用途，最多5张。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/midjourney/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"参考图片中的主体外形，生成夜晚街道场景，电影感构图\",\n  \"size\": \"16:9\",\n  \"image\": \"https://example.com/reference.jpg\",\n  \"iw\": 1,\n  \"raw\": true,\n  \"n\": 1\n}'"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/midjourney/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"参考第一张图中的主体、第二张图的配色和光线，生成一张商业海报\",\n  \"size\": \"4:5\",\n  \"images\": [\n    \"https://example.com/subject.jpg\",\n    \"https://example.com/style.jpg\"\n  ],\n  \"iw\": 1.5,\n  \"n\": 1\n}'"
      },
      {
        "type": "paragraph",
        "value": "Data URL同样放入image或images。下例中的Base64占位内容需替换为真实图片编码，MIME应与文件格式一致；不要把图片地址写进prompt来代替参考图字段。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"参考主体的形状与配色，生成明亮的商业插画\",\n  \"image\": \"data:image/png;base64,<图片文件的Base64编码>\",\n  \"size\": \"1:1\",\n  \"n\": 1\n}"
      },
      {
        "type": "heading",
        "value": "角色、风格与深度参考"
      },
      {
        "type": "table",
        "headers": [
          "参考用途",
          "图片字段",
          "权重字段与范围"
        ],
        "rows": [
          [
            "普通图片参考",
            "image或images",
            "iw：0–3"
          ],
          [
            "角色参考",
            "cref：角色参考图URL",
            "cw：0–100"
          ],
          [
            "风格参考",
            "sref：风格参考图URL",
            "sw：0–1000"
          ],
          [
            "深度参考",
            "dref：深度参考图URL",
            "dw：0–100"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "cref、sref、dref是不同用途的参考图URL字段，不是images的别名；对应权重只用于相应参考用途。没有该用途时省略图片和权重字段，不把普通参考图的5张上限解释为这些字段各自都可再传5张。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/midjourney/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"mj-v8.2\",\n  \"prompt\": \"以参考主体创作自然光下的静物海报，借鉴风格参考图的色调\",\n  \"image\": \"https://example.com/subject.jpg\",\n  \"iw\": 1,\n  \"sref\": \"https://example.com/style.jpg\",\n  \"sw\": 100,\n  \"size\": \"4:5\",\n  \"quality\": 1,\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "后续动作参数"
      },
      {
        "type": "paragraph",
        "value": "普通文生图与参考生图不填写action。需要对已有任务进行操作时，再按动作使用父任务task_id、图片索引index或原任务提供的custom_id；不要把所有动作相关字段一起发送。"
      },
      {
        "type": "table",
        "headers": [
          "action",
          "用途"
        ],
        "rows": [
          [
            "upscale",
            "放大图片"
          ],
          [
            "variation",
            "生成变体"
          ],
          [
            "high_variation",
            "高变化变体"
          ],
          [
            "low_variation",
            "低变化变体"
          ],
          [
            "remix_strong",
            "强混合重绘"
          ],
          [
            "remix_subtle",
            "轻混合重绘"
          ],
          [
            "reroll",
            "重新生成"
          ],
          [
            "zoom",
            "扩图"
          ],
          [
            "pan",
            "平移画面"
          ],
          [
            "blend",
            "融图"
          ],
          [
            "edits",
            "图片编辑"
          ],
          [
            "describe",
            "描述图片"
          ]
        ]
      },
      {
        "type": "note",
        "value": "action仅使用表中列出的值。index、custom_id、direction、zoom_ratio与dimensions未约定统一的全部取值范围；按原任务可用操作填写。文档中的left和SQUARE只是示例值，不表示完整枚举。后续动作不改变本站的任务查询方式。"
      },
      {
        "type": "heading",
        "value": "异步任务：提交、查询、保存"
      },
      {
        "type": "paragraph",
        "value": "提交接口先返回任务受理信息，不代表图片已经生成完成。任务号优先读取响应体中的`task_id`，也兼容`data.task_id`和响应头`X-NewAPI-Task-Id`；拿到任务号后使用同一个Key调用`GET /v1/tasks/{task_id}`，直到任务进入完成或失败状态。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_mj_example\",\n  \"task_id\": \"task_mj_example\",\n  \"status\": \"processing\",\n  \"result_expired\": false,\n  \"data\": {\n    \"task_id\": \"task_mj_example\",\n    \"status\": \"processing\",\n    \"progress\": \"0%\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body 'https://please.dontblockme.men/v1/tasks/task_mj_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "实际兼容位置",
          "含义与处理"
        ],
        "rows": [
          [
            "task_id",
            "顶层或data.task_id；也兼容X-NewAPI-Task-Id",
            "本次任务编号，保存后用于恢复查询。"
          ],
          [
            "status",
            "顶层或data.status",
            "`submitted`、`pending`、`processing`、`waiting`、`queued`、`running`表示继续查询。"
          ],
          [
            "完成状态",
            "顶层或data.status",
            "`completed`、`succeeded`、`success`均按成功处理。"
          ],
          [
            "失败状态",
            "顶层或data.status",
            "`failed`、`failure`、`cancelled`、`canceled`、`error`停止查询并读取错误字段。"
          ],
          [
            "image_urls",
            "data.result.data.image_urls、result.data.image_urls或data.image_urls",
            "图片URL数组；按数组顺序下载4张单图。"
          ],
          [
            "result_expired",
            "顶层或data.result_expired",
            "为`true`表示任务记录仍在但图片文件已过期，不能继续取图。"
          ],
          [
            "error_message / error_code / error.message",
            "顶层或data对象",
            "失败原因；停止等待，不要把失败当作空图片成功。"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_mj_example\",\n  \"task_id\": \"task_mj_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"data\": {\n    \"task_id\": \"task_mj_example\",\n    \"status\": \"completed\",\n    \"progress\": \"100%\",\n    \"result\": {\n      \"data\": {\n        \"image_urls\": [\n          \"/task-media/task_mj_example/media/0?expires=...&signature=...\",\n          \"/task-media/task_mj_example/media/1?expires=...&signature=...\",\n          \"/task-media/task_mj_example/media/2?expires=...&signature=...\",\n          \"/task-media/task_mj_example/media/3?expires=...&signature=...\"\n        ]\n      }\n    }\n  },\n  \"result\": {\n    \"data\": {\n      \"image_urls\": [\n        \"/task-media/task_mj_example/media/0?expires=...&signature=...\",\n        \"/task-media/task_mj_example/media/1?expires=...&signature=...\",\n        \"/task-media/task_mj_example/media/2?expires=...&signature=...\",\n        \"/task-media/task_mj_example/media/3?expires=...&signature=...\"\n      ]\n    }\n  }\n}"
      },
      {
        "type": "paragraph",
        "value": "完成响应的图片字段以`image_urls`为准。返回地址可能是带签名的相对地址，需以当前API域名补全；地址可能过期，完成后及时下载。下载图片时不要携带TokenFreeKey。"
      },
      {
        "type": "heading",
        "value": "完整示例：生成、断点恢复与图片下载"
      },
      {
        "type": "paragraph",
        "value": "将下面代码保存为`mj-v8.2.mjs`，使用Node.js 22或更新版本运行。设置`TOKENFREE_API_KEY`，执行`node mj-v8.2.mjs`；中断后执行`node mj-v8.2.mjs task.json`恢复查询。结果保存为`result.json`和`image-1.png`等。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY='YOUR_API_KEY'\nnode mj-v8.2.mjs\n\n# 中断后仅恢复查询，不重复提交。\nnode mj-v8.2.mjs task.json"
      },
      {
        "type": "paragraph",
        "value": "本示例固定使用`mj-v8.2`。自定义请求可保存为JSON文件，再设置`TOKENFREE_MJ_REQUEST`为该文件路径，请求中的`model`也填写`mj-v8.2`。恢复已有任务不会重新读取请求文件，也不会再次生成。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { Buffer } from 'node:buffer';\nimport { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst API_BASE = ORIGIN;\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\nconst model = 'mj-v8.2';\nconst success = new Set(['completed', 'succeeded', 'success']);\nconst failure = new Set(['failed', 'failure', 'cancelled', 'canceled', 'error']);\nconst retryable = new Set([408, 429, 500, 502, 503, 504]);\n\nfunction apiUrl(path) {\n  const url = new URL(path, API_BASE + '/');\n  if (url.origin !== ORIGIN || !url.pathname.startsWith('/v1/')) throw new Error('查询地址必须属于本站 API');\n  return url;\n}\nfunction resultUrl(source) {\n  if (typeof source !== 'string' || !source.trim()) throw new Error('结果图片地址为空');\n  if (/^data:image\\//i.test(source)) return source;\n  const url = new URL(source, ORIGIN + '/v1/');\n  if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new Error('结果地址必须使用HTTP或HTTPS');\n  return url;\n}\nasync function readJson(response, { allowEmpty = false } = {}) {\n  const text = await response.text();\n  if (!text.trim()) {\n    if (allowEmpty) return {};\n    throw new Error('接口未返回JSON，HTTP ' + response.status);\n  }\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回JSON，HTTP ' + response.status); }\n  const error = value.error?.message || value.error_message || value.error_code || (!response.ok && (value.detail || value.message));\n  if (!response.ok || error) throw new Error(String(error || 'HTTP ' + response.status));\n  return value;\n}\nfunction intervalFor(response, fallback = 3000) {\n  const raw = response.headers.get('Retry-After');\n  if (!raw) return fallback;\n  const seconds = Number(raw);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(raw) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : fallback;\n}\nfunction normalize(value, headerTaskId = '') {\n  const root = value && typeof value === 'object' ? value : {};\n  const data = root.data && typeof root.data === 'object' && !Array.isArray(root.data) ? root.data : root;\n  const result = data.result && typeof data.result === 'object' ? data.result : data;\n  const output = result.data && typeof result.data === 'object' && !Array.isArray(result.data) ? result.data : result;\n  const taskId = typeof data.task_id === 'string' ? data.task_id : typeof root.task_id === 'string' ? root.task_id : headerTaskId;\n  const statusValue = typeof data.status === 'string' ? data.status : typeof root.status === 'string' ? root.status : '';\n  const images = Array.isArray(output.image_urls) ? output.image_urls.filter(item => typeof item === 'string' && item.trim()) : [];\n  const error = data.error_message || root.error_message || data.error_code || root.error_code || data.error?.message || root.error?.message || '';\n  const resultExpired = data.result_expired === true || root.result_expired === true;\n  return { taskId, status: statusValue.toLowerCase(), images, error: String(error || ''), resultExpired, raw: value };\n}\nfunction checkTerminal(parsed) {\n  if (failure.has(parsed.status)) throw new Error(parsed.error || parsed.status);\n  if (parsed.resultExpired) throw new Error('任务已完成，但图片结果已过期，请重新提交生成');\n  if (success.has(parsed.status)) {\n    if (!parsed.images.length) throw new Error('任务完成但没有image_urls，请检查原始响应');\n    return true;\n  }\n  return false;\n}\n\nlet parsed;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 恢复已有任务时仅查询，不再次提交或生成。\n  const saved = JSON.parse(await readFile(process.argv[2], 'utf8'));\n  parsed = normalize(saved);\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复查询，或在新目录生成');\n  const request = process.env.TOKENFREE_MJ_REQUEST\n    ? JSON.parse(await readFile(process.env.TOKENFREE_MJ_REQUEST, 'utf8'))\n    : { model, prompt: '浅色背景上的橘子汽水，柔和光线，细腻插画', size: '1:1', raw: false, n: 1 };\n  if (request.model !== model) throw new Error('请求文件中的模型名称必须为mj-v8.2');\n  if (request.n != null && request.n !== 1) throw new Error('MJ的n固定为1');\n  // 只校验文档已经明确的输入限制，不改写任务返回或静默截断提示词。\n  const promptLength = typeof request.prompt === 'string' ? Array.from(request.prompt).length : 0;\n  if (promptLength < 1 || promptLength > 1023) throw new Error('prompt长度必须为1–1023个字符');\n  if (request.image != null && request.images != null) throw new Error('image与images请选择一种');\n  const validReference = value => {\n    if (typeof value !== 'string' || !value.trim()) return false;\n    if (/^data:image\\/[^;,]+;base64,.+/i.test(value)) return true;\n    try { return ['https:', 'http:'].includes(new URL(value).protocol); }\n    catch { return false; }\n  };\n  if (request.image != null && !validReference(request.image)) throw new Error('image必须是图片URL或完整Data URL');\n  if (request.images != null && (!Array.isArray(request.images) || request.images.length < 1 || request.images.length > 5 || !request.images.every(validReference))) {\n    throw new Error('images必须包含1–5张图片URL或完整Data URL');\n  }\n  const ranges = { stylize: [0, 1000], chaos: [0, 100], weird: [0, 3000], iw: [0, 3], cw: [0, 100], sw: [0, 1000], dw: [0, 100] };\n  for (const [field, [min, max]] of Object.entries(ranges)) {\n    if (request[field] != null && (typeof request[field] !== 'number' || !Number.isFinite(request[field]) || request[field] < min || request[field] > max)) {\n      throw new Error(field + '必须为' + min + '–' + max + '之间的数值');\n    }\n  }\n  if (request.quality != null && ![0.25, 0.5, 1, 2].includes(request.quality)) throw new Error('quality必须为数值0.25、0.5、1或2');\n  for (const field of ['raw', 'hd', 'tile']) {\n    if (request[field] != null && typeof request[field] !== 'boolean') throw new Error(field + '必须为JSON布尔值');\n  }\n  const response = await fetch(API_BASE + '/v1/midjourney/generations', {\n    method: 'POST', headers: { ...headers, 'Content-Type': 'application/json' },\n    body: JSON.stringify(request), redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  const submitted = normalize(await readJson(response, { allowEmpty: true }), response.headers.get('X-NewAPI-Task-Id') || '');\n  if (submitted.error) throw new Error(submitted.error);\n  if (checkTerminal(submitted)) {\n    parsed = submitted;\n  } else {\n    if (!submitted.taskId) throw new Error('未返回task_id，请保留响应并核对任务记录，不要自动重新生成');\n    parsed = { taskId: submitted.taskId, status: '', images: [], error: '', resultExpired: false, raw: { task_id: submitted.taskId } };\n    await writeFile('task.json', JSON.stringify({ task_id: submitted.taskId }, null, 2), { flag: 'wx' });\n    console.log('任务已保存：' + submitted.taskId);\n    interval = intervalFor(response, interval);\n  }\n}\n\nif (!checkTerminal(parsed)) {\n  if (typeof parsed.taskId !== 'string' || !parsed.taskId) throw new Error('任务文件缺少task_id');\n  const pollUrl = apiUrl('/v1/tasks/' + encodeURIComponent(parsed.taskId));\n  const deadline = Date.now() + 30 * 60 * 1000;\n  while (true) {\n    if (Date.now() + interval >= deadline) throw new Error('等待已结束，可传入task.json恢复查询');\n    await delay(interval);\n    let response;\n    try {\n      response = await fetch(pollUrl, { headers, redirect: 'error', signal: AbortSignal.timeout(30000), cache: 'no-store' });\n    } catch {\n      // 网络错误只重试查询，禁止自动重发生成请求。\n      interval = Math.min(interval * 2, 30000);\n      continue;\n    }\n    if (retryable.has(response.status)) {\n      await response.body?.cancel();\n      interval = Math.max(intervalFor(response, interval), Math.min(interval * 2, 30000));\n      continue;\n    }\n    parsed = normalize(await readJson(response), parsed.taskId);\n    interval = intervalFor(response, interval);\n    if (checkTerminal(parsed)) break;\n    if (!parsed.status) throw new Error('查询响应缺少status，请保留原始响应排查');\n  }\n}\n\nawait writeFile('result.json', JSON.stringify(parsed.raw, null, 2));\nfor (const [index, source] of parsed.images.entries()) {\n  const name = 'image-' + (index + 1);\n  if (/^data:image\\//i.test(source)) {\n    const match = source.match(/^data:(image\\/(?:png|jpeg|webp));base64,([A-Za-z0-9+/=]+)$/i);\n    if (!match) throw new Error('Data URL必须是PNG、JPEG或WebP的Base64图片');\n    const ext = match[1].toLowerCase() === 'image/jpeg' ? 'jpg' : match[1].split('/')[1];\n    await writeFile(name + '.' + ext, Buffer.from(match[2], 'base64'));\n    continue;\n  }\n  const response = await fetch(resultUrl(source), { redirect: 'error', signal: AbortSignal.timeout(120000) });\n  if (!response.ok || !response.body) throw new Error('图片下载失败，HTTP ' + response.status);\n  const mime = (response.headers.get('Content-Type') || '').split(';')[0].trim();\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp' })[mime];\n  if (!ext) throw new Error('图片下载返回了非图片内容：' + mime);\n  try {\n    await pipeline(Readable.fromWeb(response.body), createWriteStream(name + '.' + ext + '.part'));\n    await rename(name + '.' + ext + '.part', name + '.' + ext);\n  } catch (error) {\n    await rm(name + '.' + ext + '.part', { force: true });\n    throw error;\n  }\n}\nconsole.log('已保存' + parsed.images.length + '张单图');\n"
      },
      {
        "type": "heading",
        "value": "常见错误与处理"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "鉴权失败",
            "确认使用TokenFreeKey，并对提交和查询使用同一个Key。"
          ],
          [
            "模型不存在",
            "请求的`model`必须填写`mj-v8.2`。"
          ],
          [
            "提交后取不到任务号",
            "按顶层`task_id`、`data.task_id`、`X-NewAPI-Task-Id`的顺序读取，保留原始响应后再排查。"
          ],
          [
            "查询响应没有data.status",
            "同时兼容顶层`status`和`data.status`，不要只读取固定一层。"
          ],
          [
            "完成但没有图片",
            "读取`image_urls`所在的`result.data`层级；同时检查`result_expired`，保留原始响应排查。"
          ],
          [
            "比例不合法",
            "填写本页比例枚举；`size`不是GPT Image的像素尺寸字段。"
          ],
          [
            "参考图加载失败",
            "确认URL无需登录即可读取，或提供完整data:image/...内容；检查张数和顺序。"
          ],
          [
            "查询超时或网络断开",
            "保留task.json恢复查询，不要自动重新发起生成。"
          ]
        ]
      },
      {
        "type": "note",
        "value": "价格以模型列表与实际账单为准；不要依据返回图片数组长度推算其他模型的价格。"
      }
    ]
  },
  {
    "id": "video",
    "label": "Wan 视频",
    "title": "Wan 3.0 视频生成 API",
    "lead": "Wan 3.0文生视频、图生视频及音视频参考：型号与分辨率选择、完整参数、异步任务查询、恢复与下载。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "1. 基础信息与接入流程"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "说明"
        ],
        "rows": [
          [
            "API Base URL",
            "`https://please.dontblockme.men`"
          ],
          [
            "创建视频",
            "`POST /v1/videos`"
          ],
          [
            "查询视频",
            "`GET /v1/videos/{id}`"
          ],
          [
            "下载视频",
            "`GET /v1/videos/{id}/content`"
          ],
          [
            "主要接入模式",
            "异步：创建 → 保存id → 查询 → completed → 下载"
          ],
          [
            "模型权限",
            "以当前Key在`GET /v1/models`返回的名称为准"
          ],
          [
            "计费方式",
            "按生成秒数计费；实际单价由当前型号与账户分组决定"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "输出分辨率",
          "时长",
          "参考图片",
          "参考视频与音频"
        ],
        "rows": [
          [
            "`wan-3.0`",
            "`720p`",
            "1–30秒（整数）",
            "最多2张",
            "支持video_urls与audio_urls，可与图片组合"
          ],
          [
            "`wan-3.0-1080p`",
            "最高`1080p`；可选择`720p`或`1080p`",
            "1–30秒（整数）",
            "最多2张",
            "支持video_urls与audio_urls，可与图片组合"
          ]
        ]
      },
      {
        "type": "note",
        "value": "两个型号使用相同的创建、查询、下载接口及素材字段。wan-3.0-1080p表示最高支持1080p，并非固定输出1080p；请用resolution明确选择输出分辨率。下方示例分别展示720p和1080p。"
      },
      {
        "type": "heading",
        "value": "2. 鉴权与请求格式"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "http",
        "value": "Authorization: Bearer <API_KEY>\nContent-Type: application/json"
      },
      {
        "type": "paragraph",
        "value": "创建使用JSON，查询和下载只需要Authorization。cURL示例采用Bash换行语法；PowerShell请使用curl.exe并调整变量及换行。Key放在服务端或本机环境变量中，不写入公开网页。"
      },
      {
        "type": "heading",
        "value": "3. 请求参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "必填",
          "规则"
        ],
        "rows": [
          [
            "`model`",
            "string",
            "是",
            "`wan-3.0`；高清型号使用完整名称`wan-3.0-1080p`"
          ],
          [
            "`prompt`",
            "string",
            "是",
            "描述主体、动作、场景、镜头和素材用途"
          ],
          [
            "`duration`",
            "integer",
            "是",
            "两个型号均使用1–30的整数，单位秒；显式传入duration，不混传seconds"
          ],
          [
            "`resolution`",
            "string",
            "建议显式传入",
            "wan-3.0使用`720p`；wan-3.0-1080p可使用`720p`或`1080p`，最高1080p。省略时由服务端决定，不以模型后缀代替该字段"
          ],
          [
            "`aspect_ratio`",
            "string",
            "否",
            "例如`16:9`、`9:16`、`1:1`；是比例，不是像素尺寸"
          ],
          [
            "`image_urls`",
            "string[]",
            "否",
            "公网参考图片URL数组，两个型号均最多2张；在prompt中说明各张图片的作用"
          ],
          [
            "`video_urls`",
            "string[]",
            "否",
            "公网参考视频URL数组，可用于参考镜头、动作与剧情节奏"
          ],
          [
            "`audio_urls`",
            "string[]",
            "否",
            "公网参考音频URL数组，可用于参考节拍与声音；与图片或视频一起提交时，在prompt中描述配合方式"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "推荐字段",
          "旧字段别名"
        ],
        "rows": [
          [
            "`image_urls`",
            "`images`、`reference_image_urls`"
          ],
          [
            "`video_urls`",
            "`videos`、`reference_videos`"
          ],
          [
            "`audio_urls`",
            "`audios`、`reference_audios`"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "新接入只使用推荐字段，不混传同类素材的新旧名称。旧工具可能生成input_reference，请改用本页的image_urls；字段能被网关转发不代表上游一定采用它。seed、generate_audio等未在本页承诺的参数，不作为基础接入的必要字段。"
      },
      {
        "type": "heading",
        "value": "4. 文生视频"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍，镜头自然推进\",\n  \"duration\": 10,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "paragraph",
        "value": "这个请求不添加Prefer: respond-async。视频接口本身就是异步接口：创建返回的是视频任务，不是MP4文件。保存返回的id，再按第8节查询。"
      },
      {
        "type": "paragraph",
        "value": "1080p文生视频：使用高清型号并显式填写resolution。下例生成10秒横屏视频。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"wan-3.0-1080p\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍，镜头自然推进\",\n  \"duration\": 10,\n  \"resolution\": \"1080p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "paragraph",
        "value": "同一个高清型号也可以请求720p。分辨率由resolution选择，查询和下载流程保持一致："
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"wan-3.0-1080p\",\n  \"prompt\": \"海边公路上一辆蓝色轿车平稳行驶，镜头稳定跟拍\",\n  \"duration\": 10,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}"
      },
      {
        "type": "heading",
        "value": "5. 单图、双图与30秒图生视频"
      },
      {
        "type": "paragraph",
        "value": "单图参考：把图片地址放入image_urls，并在prompt中明确要求保留的身份、商品外观和构图。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"保持参考图中商品的造型和颜色，镜头从正面缓慢移动到侧面，光线柔和，不新增文字\",\n  \"duration\": 10,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"9:16\",\n  \"image_urls\": [\n    \"https://cdn.example.com/product.jpg\"\n  ]\n}'"
      },
      {
        "type": "paragraph",
        "value": "双图参考：数组顺序就是提示词中“第一张图”“第二张图”的顺序。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"第一张图用于人物身份，第二张图用于服装设计，人物自然转身展示衣服，镜头稳定\",\n  \"duration\": 15,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"9:16\",\n  \"image_urls\": [\n    \"https://cdn.example.com/person.jpg\",\n    \"https://cdn.example.com/outfit.jpg\"\n  ]\n}"
      },
      {
        "type": "paragraph",
        "value": "两个型号均支持最长30秒。较长镜头可以在prompt中按时间段组织动作；一次请求创建一个任务。高清输出时使用wan-3.0-1080p并选择resolution为1080p。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"保留参考图主体。前10秒缓慢推近展示全景，中间10秒展示商品细节，最后10秒回到稳定正面构图，镜头连贯自然\",\n  \"duration\": 30,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://cdn.example.com/product.jpg\"\n  ]\n}"
      },
      {
        "type": "paragraph",
        "value": "1080p双图参考：两张图片分别提供人物与服装信息；同样支持最长30秒。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"wan-3.0-1080p\",\n  \"prompt\": \"第一张图用于人物身份，第二张图用于服装设计。前10秒展示人物与环境，中间10秒自然行走展示衣服细节，最后10秒稳定展示正面造型。保持身份与服装一致，镜头衔接自然。\",\n  \"duration\": 30,\n  \"resolution\": \"1080p\",\n  \"aspect_ratio\": \"9:16\",\n  \"image_urls\": [\n    \"https://cdn.example.com/person.jpg\",\n    \"https://cdn.example.com/outfit.jpg\"\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "6. 视频、音频参考与素材要求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"保持参考图人物外观，参考视频的镜头节奏，让人物动作与参考音频节拍自然配合\",\n  \"duration\": 15,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://cdn.example.com/person.jpg\"\n  ],\n  \"video_urls\": [\n    \"https://cdn.example.com/motion.mp4\"\n  ],\n  \"audio_urls\": [\n    \"https://cdn.example.com/rhythm.mp3\"\n  ]\n}'"
      },
      {
        "type": "list",
        "items": [
          "URL必须能被生成服务直接读取，使用公开的HTTP或HTTPS媒体文件地址。",
          "不依赖Cookie、登录状态、Referer或额外请求头；地址直接返回媒体，不返回网页或预览页。",
          "图片、视频和音频的Content-Type应与实际内容一致。",
          "签名地址在排队、提交及生成期间都要有效，不使用即将过期的链接。",
          "本地磁盘路径不能直接传入JSON；先上传到可公开读取的存储。",
          "不要把Base64或data:字符串放入URL数组。无需为接入本协议增加视频转码或Base64包装。",
          "请求中只传本次需要的素材类型；同类字段不要重复传别名。"
        ]
      },
      {
        "type": "heading",
        "value": "7. 创建任务响应"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"wan-3.0\",\n  \"status\": \"queued\",\n  \"progress\": 0,\n  \"created_at\": 1790900000\n}"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "说明"
        ],
        "rows": [
          [
            "`id`",
            "string",
            "本站视频任务编号，后续查询与下载使用此值"
          ],
          [
            "`task_id`",
            "string，可选",
            "旧客户端可能收到的兼容字段；优先使用id，不要求两者同时存在"
          ],
          [
            "`object`",
            "string",
            "`video`"
          ],
          [
            "`model`",
            "string",
            "本次请求的公开模型名称"
          ],
          [
            "`status`",
            "string",
            "通常为queued；具体含义见状态表"
          ],
          [
            "`progress`",
            "integer",
            "0—100的进度提示；不是带百分号的字符串，也不代表剩余秒数"
          ],
          [
            "`created_at`",
            "integer",
            "Unix时间戳，单位秒"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "创建成功接收2xx响应（通常200或202），同时校验JSON中的id。HTTP成功只代表任务已受理，不代表视频生成完成。不要把上游私有编号替代本站id。"
      },
      {
        "type": "heading",
        "value": "8. 查询任务与状态机"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/videos/task_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"wan-3.0\",\n  \"status\": \"in_progress\",\n  \"progress\": 45,\n  \"created_at\": 1790900000\n}"
      },
      {
        "type": "code",
        "lang": "text",
        "value": "queued → in_progress → completed → GET /v1/videos/{id}/content\n                    ↘ failed    → 记录错误并停止查询"
      },
      {
        "type": "table",
        "headers": [
          "status",
          "含义",
          "客户端动作"
        ],
        "rows": [
          [
            "`queued`",
            "等待处理",
            "继续查询相同id"
          ],
          [
            "`in_progress`",
            "正在提交或生成",
            "继续查询，不重新创建"
          ],
          [
            "`completed`",
            "生成成功",
            "开始下载文件"
          ],
          [
            "`failed`",
            "生成失败",
            "保存响应和错误信息，停止查询"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "建议从5秒一次开始查询；遇到Retry-After遵循服务端提示，429和临时5xx只重试查询并退避。设置本地等待期限，超时后保留id继续查询，而不是再次POST创建。未知状态不要当成成功。"
      },
      {
        "type": "heading",
        "value": "9. 成功响应与结果字段"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"wan-3.0\",\n  \"status\": \"completed\",\n  \"progress\": 100,\n  \"created_at\": 1790900000,\n  \"completed_at\": 1790900120\n}"
      },
      {
        "type": "paragraph",
        "value": "completed_at是生成完成时间，单位秒。不同协议可能附带video_urls、metadata或其他扩展字段，但这些不是本站客户端必须依赖的契约。本站不会保证暴露上游私有下载地址；统一使用内容下载接口。"
      },
      {
        "type": "heading",
        "value": "10. 下载与保存视频"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 300 \\\n  'https://please.dontblockme.men/v1/videos/task_example/content' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.mp4"
      },
      {
        "type": "list",
        "items": [
          "只在status为completed后下载，响应正文是媒体字节，不是JSON。",
          "内容接口需要Bearer鉴权，直接粘贴到浏览器地址栏可能返回401。",
          "保存到自己的存储；任务记录存在不代表归档文件永久有效，过期文件可能返回410。",
          "程序采用流式写文件，避免把完整视频一次性加载到内存。",
          "下载网络错误可重试同一文件；不要因此重新生成视频。"
        ]
      },
      {
        "type": "heading",
        "value": "11. 失败响应与排错"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"wan-3.0\",\n  \"status\": \"failed\",\n  \"progress\": 100,\n  \"created_at\": 1790900000,\n  \"error\": {\n    \"message\": \"参考素材读取失败，请检查媒体地址\",\n    \"code\": \"generation_failed\"\n  }\n}"
      },
      {
        "type": "paragraph",
        "value": "此处为失败结构示意，具体message和code由实际错误决定，error字段可能省略。始终以status判断是否失败；HTTP 200的查询响应也可能携带failed。"
      },
      {
        "type": "table",
        "headers": [
          "HTTP状态",
          "可能原因",
          "处理方式"
        ],
        "rows": [
          [
            "400",
            "参数类型、时长、分辨率或素材地址不匹配",
            "核对模型表与字段，保存错误正文"
          ],
          [
            "401",
            "Key无效或缺失",
            "检查Bearer请求头及当前Key"
          ],
          [
            "402 / 403",
            "余额、额度或权限限制",
            "结合error中的原因检查账户余额、令牌与分组权限"
          ],
          [
            "404",
            "路径错误、任务不存在或不属于当前账户",
            "核对完整id和创建时使用的账户"
          ],
          [
            "400 / 409",
            "视频未完成就请求下载",
            "先查询到completed；不同下载入口的状态码可能不同"
          ],
          [
            "410",
            "归档结果已过期",
            "及时保存文件；先确认结果确实过期再决定是否重新生成"
          ],
          [
            "429",
            "限流或并发过高",
            "尊重Retry-After；降低查询频率"
          ],
          [
            "502 / 503 / 504",
            "上游、代理或网络临时异常",
            "查询请求可退避重试；创建超时先确认原任务是否已经受理"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "12. 网关异步模式：与Videos任务的区别"
      },
      {
        "type": "table",
        "headers": [
          "模式",
          "创建方式",
          "编号与查询",
          "完成状态"
        ],
        "rows": [
          [
            "Videos任务（本页默认）",
            "不添加异步请求头",
            "`task_...` → `/v1/videos/{id}`",
            "`completed`"
          ],
          [
            "网关异步任务",
            "添加`Prefer: respond-async`或`?async=true`",
            "`async_...` → 返回的`poll_url`，通常`/v1/tasks/{id}`",
            "`succeeded`"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "两种方式均能异步完成生成，但返回对象与状态机不同。按创建响应选择一套流程，不把completed与succeeded、task_编号与async_编号混用。下面给出网关模式的完整请求和结果结构。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"wan-3.0\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍，镜头自然推进\",\n  \"duration\": 10,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"video_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"wan-3.0\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/videos\"\n}"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "table",
        "headers": [
          "网关status",
          "含义"
        ],
        "rows": [
          [
            "pending / processing / waiting",
            "排队、处理或等待视频任务完成，继续查询"
          ],
          [
            "succeeded",
            "已完成，检查result_expired并读取media"
          ],
          [
            "failed / cancelled",
            "终止查询，读取error.message"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"id\": \"task_example\",\n    \"object\": \"video\",\n    \"model\": \"wan-3.0\",\n    \"status\": \"completed\",\n    \"progress\": 100,\n    \"created_at\": 1790900000,\n    \"completed_at\": 1790900120\n  },\n  \"media\": [\n    {\n      \"url\": \"/v1/tasks/async_example/media/0\",\n      \"kind\": \"video\",\n      \"content_type\": \"video/mp4\"\n    }\n  ]\n}"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 300 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" --output result.mp4"
      },
      {
        "type": "paragraph",
        "value": "media[].url需要鉴权；preview_url若存在是临时展示地址。result_expired为true时停止下载。第13、14节示例使用Videos任务接口，第16节示例使用本节的网关任务接口，任选一种模式接入。"
      },
      {
        "type": "heading",
        "value": "13. Node.js完整示例"
      },
      {
        "type": "paragraph",
        "value": "示例默认调用wan-3.0、输出720p。调用高清型号时分别设置TOKENFREE_VIDEO_MODEL与TOKENFREE_VIDEO_RESOLUTION；两个完整示例均支持这两个环境变量。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_VIDEO_MODEL=\"wan-3.0-1080p\"\nexport TOKENFREE_VIDEO_RESOLUTION=\"1080p\"\n# 同一高清型号需要720p时，将分辨率改为720p。"
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_VIDEO_MODEL = \"wan-3.0-1080p\"\n$env:TOKENFREE_VIDEO_RESOLUTION = \"1080p\""
      },
      {
        "type": "paragraph",
        "value": "Node.js 22或更新版本，无额外依赖。保存为video.mjs，设置TOKENFREE_API_KEY，在新目录执行。此示例使用默认Videos任务，不加Prefer请求头。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node video.mjs\n# 恢复同一任务\nnode video.mjs video-task.json"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\nconst taskFile = process.argv[2] || 'video-task.json';\n// 高清型号可以选择720p或1080p，模型名称不等于固定输出分辨率。\nconst model = process.env.TOKENFREE_VIDEO_MODEL || 'wan-3.0';\nconst resolution = process.env.TOKENFREE_VIDEO_RESOLUTION || '720p';\nif (!['wan-3.0', 'wan-3.0-1080p'].includes(model)) throw new Error('未知Wan模型：' + model);\nconst resolutions = model === 'wan-3.0' ? ['720p'] : ['720p', '1080p'];\nif (!resolutions.includes(resolution)) throw new Error('当前型号不支持此分辨率：' + resolution);\nconst request = {\n  model,\n  prompt: '晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍',\n  duration: 10, resolution, aspect_ratio: '16:9'\n  // 图生视频时增加 image_urls: ['https://你的域名/参考图.jpg']。\n};\n\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction retryAfter(response, fallback = 5000) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return fallback;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms >= 0 ? Math.max(ms, 1000) : fallback;\n}\n\nlet submitted;\nlet interval = 5000;\nif (process.argv[2]) {\n  // 恢复时只读取已保存的编号，不再次创建收费任务。\n  submitted = JSON.parse(await readFile(taskFile, 'utf8'));\n} else {\n  if (existsSync(taskFile)) throw new Error('已有video-task.json，请传入此文件恢复，或在新目录创建新任务');\n  // 创建请求只发送一次；连接超时也不自动重发。\n  const response = await fetch(ORIGIN + '/v1/videos', {\n    method: 'POST', headers: { ...headers, 'Content-Type': 'application/json' },\n    body: JSON.stringify(request), redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (!submitted.id && !submitted.task_id) throw new Error('响应缺少视频任务id');\n  console.log('请保留任务编号：', submitted.id || submitted.task_id);\n  await writeFile(taskFile, JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  interval = retryAfter(response);\n}\nconst id = submitted.id || submitted.task_id;\nif (typeof id !== 'string' || !id || id.startsWith('async_')) {\n  throw new Error('需要Videos任务id；async_编号请使用网关任务示例');\n}\nconst endpoint = ORIGIN + '/v1/videos/' + encodeURIComponent(id);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() + interval < deadline) {\n  await delay(interval);\n  let response;\n  try {\n    response = await fetch(endpoint, { headers, redirect: 'error', signal: AbortSignal.timeout(30000) });\n  } catch {\n    // 网络故障只重试查询，不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (response.status === 429 || response.status >= 500) {\n    interval = Math.max(retryAfter(response), Math.min(interval * 2, 30000));\n    await response.body?.cancel();\n    continue;\n  }\n  task = await readJson(response);\n  interval = retryAfter(response);\n  console.log('任务状态：', task.status, '进度：', task.progress ?? '未提供');\n  if (task.status === 'completed') break;\n  if (task.status === 'failed') throw new Error(task.error?.message || '视频生成失败：' + id);\n  if (!['queued', 'in_progress'].includes(task.status)) throw new Error('未知视频状态：' + task.status);\n}\nif (task?.status !== 'completed') throw new Error('本地等待结束；传入任务文件继续查询，服务端任务不会因此取消');\nawait writeFile('video-result.json', JSON.stringify(task, null, 2));\n\n// 流式下载；完成前使用临时扩展名，避免把不完整视频误认为成品。\nconst response = await fetch(endpoint + '/content', {\n  headers, redirect: 'error', signal: AbortSignal.timeout(300000)\n});\nif (!response.ok) throw new Error('视频下载失败，HTTP ' + response.status + '；可以用原任务文件重试');\nif (!response.body) throw new Error('下载响应缺少文件内容');\nconst mime = (response.headers.get('Content-Type') || '').split(';')[0];\nif (mime && !mime.startsWith('video/') && mime !== 'application/octet-stream') {\n  throw new Error('下载返回的不是视频：' + mime);\n}\ntry {\n  await pipeline(Readable.fromWeb(response.body), createWriteStream('result.mp4.part'));\n  await rename('result.mp4.part', 'result.mp4');\n} catch (error) {\n  await rm('result.mp4.part', { force: true });\n  throw error;\n}\nconsole.log('已保存 result.mp4');\n"
      },
      {
        "type": "heading",
        "value": "14. Python完整示例"
      },
      {
        "type": "paragraph",
        "value": "Python示例同样读取TOKENFREE_VIDEO_MODEL与TOKENFREE_VIDEO_RESOLUTION。创建只提交一次，保存video-task.json后可恢复查询；完成后分块下载，不把整个视频加载到内存。"
      },
      {
        "type": "paragraph",
        "value": "Python 3.10或更新版本，仅使用标准库，无需pip安装。保存为video.py，与Node.js示例任选一个运行，不要为同一目标同时运行两个创建示例。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "python video.py\n# 恢复同一任务\npython video.py video-task.json"
      },
      {
        "type": "code",
        "lang": "python",
        "value": "import json\nimport os\nimport sys\nimport time\nimport urllib.error\nimport urllib.parse\nimport urllib.request\nfrom datetime import datetime, timezone\nfrom email.utils import parsedate_to_datetime\nfrom pathlib import Path\n\nORIGIN = 'https://please.dontblockme.men'\nKEY = os.environ.get('TOKENFREE_API_KEY')\nif not KEY:\n    raise SystemExit('请设置 TOKENFREE_API_KEY')\nTASK_FILE = Path(sys.argv[1] if len(sys.argv) > 1 else 'video-task.json')\n# 高清型号按请求选择分辨率，支持720p或1080p。\nMODEL = os.environ.get('TOKENFREE_VIDEO_MODEL') or 'wan-3.0'\nRESOLUTION = os.environ.get('TOKENFREE_VIDEO_RESOLUTION') or '720p'\nif MODEL not in ('wan-3.0', 'wan-3.0-1080p'):\n    raise SystemExit('未知Wan模型：' + MODEL)\nRESOLUTIONS = ('720p',) if MODEL == 'wan-3.0' else ('720p', '1080p')\nif RESOLUTION not in RESOLUTIONS:\n    raise SystemExit('当前型号不支持此分辨率：' + RESOLUTION)\nBODY = {\n    'model': MODEL,\n    'prompt': '晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍',\n    'duration': 10, 'resolution': RESOLUTION, 'aspect_ratio': '16:9',\n    # 图生视频时增加 image_urls: ['https://你的域名/参考图.jpg']。\n}\n\n\nclass NoRedirect(urllib.request.HTTPRedirectHandler):\n    # 不向跳转后的其他地址转发鉴权信息。\n    def redirect_request(self, req, fp, code, msg, headers, newurl):\n        return None\n\n\nOPENER = urllib.request.build_opener(NoRedirect())\n\n\ndef open_request(path, data=None, timeout=30):\n    headers = {'Authorization': 'Bearer ' + KEY}\n    if data is not None:\n        headers['Content-Type'] = 'application/json'\n    request = urllib.request.Request(ORIGIN + path, data=data, headers=headers)\n    return OPENER.open(request, timeout=timeout)\n\n\ndef retry_after(headers, fallback=5):\n    value = headers.get('Retry-After')\n    if not value:\n        return fallback\n    try:\n        return max(float(value), 1)\n    except ValueError:\n        try:\n            return max((parsedate_to_datetime(value) - datetime.now(timezone.utc)).total_seconds(), 1)\n        except (TypeError, ValueError, OverflowError):\n            return fallback\n\n\ndef main():\n    interval = 5\n    if len(sys.argv) > 1:\n        # 传入保存的任务文件恢复查询，不创建新任务。\n        submitted = json.loads(TASK_FILE.read_text(encoding='utf-8'))\n    else:\n        if TASK_FILE.exists():\n            raise RuntimeError('已有video-task.json，请传入此文件恢复，或在新目录创建新任务')\n        data = json.dumps(BODY, ensure_ascii=False).encode('utf-8')\n        # 创建只尝试一次；超时不等于服务端没有受理。\n        with open_request('/v1/videos', data, timeout=120) as response:\n            submitted = json.load(response)\n            interval = retry_after(response.headers)\n        task_id = submitted.get('id') or submitted.get('task_id')\n        if not task_id:\n            raise RuntimeError('响应缺少视频任务id')\n        print('请保留任务编号：', task_id)\n        with TASK_FILE.open('x', encoding='utf-8') as file:\n            json.dump(submitted, file, ensure_ascii=False, indent=2)\n    task_id = submitted.get('id') or submitted.get('task_id')\n    if not isinstance(task_id, str) or not task_id or task_id.startswith('async_'):\n        raise RuntimeError('需要Videos任务id；async_编号请使用网关任务示例')\n    endpoint = '/v1/videos/' + urllib.parse.quote(task_id, safe='')\n    deadline = time.monotonic() + 30 * 60\n    task = {}\n    while time.monotonic() + interval < deadline:\n        time.sleep(interval)\n        try:\n            with open_request(endpoint) as response:\n                task = json.load(response)\n                interval = retry_after(response.headers)\n        except urllib.error.HTTPError as error:\n            if error.code == 429 or error.code >= 500:\n                interval = max(retry_after(error.headers), min(interval * 2, 30))\n                error.close()\n                continue\n            raise\n        except (urllib.error.URLError, TimeoutError, ConnectionError):\n            interval = min(interval * 2, 30)\n            continue\n        print('任务状态：', task.get('status'), '进度：', task.get('progress', '未提供'))\n        if task.get('status') == 'completed':\n            break\n        if task.get('status') == 'failed':\n            raise RuntimeError((task.get('error') or {}).get('message') or '视频生成失败：' + task_id)\n        if task.get('status') not in ('queued', 'in_progress'):\n            raise RuntimeError('未知视频状态：' + str(task.get('status')))\n    if task.get('status') != 'completed':\n        raise RuntimeError('本地等待结束；传入任务文件继续查询，服务端任务不会因此取消')\n    Path('video-result.json').write_text(json.dumps(task, ensure_ascii=False, indent=2), encoding='utf-8')\n    partial = Path('result.mp4.part')\n    try:\n        # 固定大小分块下载，不把整个视频加载到内存。\n        with open_request(endpoint + '/content', timeout=300) as response:\n            mime = response.headers.get('Content-Type', '').split(';')[0]\n            if mime and not mime.startswith('video/') and mime != 'application/octet-stream':\n                raise RuntimeError('下载返回的不是视频：' + mime)\n            with partial.open('wb') as file:\n                while True:\n                    chunk = response.read(1024 * 1024)\n                    if not chunk:\n                        break\n                    file.write(chunk)\n        partial.replace('result.mp4')\n    finally:\n        partial.unlink(missing_ok=True)\n    print('已保存 result.mp4')\n\n\nif __name__ == '__main__':\n    try:\n        main()\n    except urllib.error.HTTPError as error:\n        # 输出有限长度的错误正文，避免把大型非JSON响应写满终端。\n        print('HTTP', error.code, error.read(4096).decode('utf-8', errors='replace'), file=sys.stderr)\n        error.close()\n        sys.exit(1)\n    except Exception as error:\n        print(str(error), file=sys.stderr)\n        sys.exit(1)\n"
      },
      {
        "type": "note",
        "value": "两个示例立即保存video-task.json，等待completed后流式写入result.mp4；失败停止，429和临时查询故障退避。30分钟是客户端等待期限，不取消服务端任务。下载失败可用原任务文件重试，不重新生成。Python的timeout用于阻塞网络操作，Node.js的AbortSignal用于请求超时。"
      },
      {
        "type": "heading",
        "value": "15. 重试、计费与接入检查"
      },
      {
        "type": "list",
        "items": [
          "生成费用按平台实际价格、时长和账户分组结算，不把文档示例中的时长当作固定价格。",
          "POST创建不做自动重试。连接断开、504或本地超时，不证明服务端没有接收任务。",
          "收到id后立即落盘，后续继续查询这个id。查询和下载的重试不应再触发创建。",
          "只有确认原任务失败并处理原因后，才考虑创建新任务。不要承诺所有失败都会按同一种方式退款。",
          "向售后提供模型、任务id、请求时间、HTTP状态和去掉Key的参数，避免发送完整API Key。"
        ]
      },
      {
        "type": "heading",
        "value": "16. 网关模式完整示例"
      },
      {
        "type": "paragraph",
        "value": "若希望图片与视频共用网关任务查询方式，使用以下Node.js示例。保存为video-gateway.mjs，在新目录设置TOKENFREE_API_KEY后运行。此示例添加Prefer: respond-async，并通过返回的poll_url查询到succeeded；不要把task.json与Videos模式的video-task.json混用。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node video-gateway.mjs\n# 恢复同一网关任务\nnode video-gateway.mjs task.json"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n// 分别选择公开模型与输出分辨率，高清型号最高支持1080p。\nconst model = process.env.TOKENFREE_VIDEO_MODEL || 'wan-3.0';\nconst resolution = process.env.TOKENFREE_VIDEO_RESOLUTION || '720p';\nif (!['wan-3.0', 'wan-3.0-1080p'].includes(model)) throw new Error('未知Wan模型：' + model);\nconst resolutions = model === 'wan-3.0' ? ['720p'] : ['720p', '1080p'];\nif (!resolutions.includes(resolution)) throw new Error('当前型号不支持此分辨率：' + resolution);\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/videos', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model, prompt: '海边公路上一辆蓝色轿车平稳行驶', duration: 10, resolution, aspect_ratio: '16:9' }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node video-gateway.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "17. 常见问题"
      },
      {
        "type": "list",
        "items": [
          "参考素材不生效：使用image_urls、video_urls或audio_urls，确认地址直接返回媒体文件，并在提示词中明确素材用途。",
          "时长或清晰度不符合预期：核对完整模型名称、duration、resolution和aspect_ratio，不混用其他模型的参数。",
          "下载返回401：内容接口和media[].url需要Bearer鉴权，直接在浏览器地址栏打开不会自动带上Key。",
          "排查时提供模型、任务id、请求时间、HTTP状态及去掉Key的参数，不发送完整Key。"
        ]
      },
      {
        "type": "links",
        "items": [
          {
            "id": "image2",
            "label": "GPT Image"
          },
          {
            "id": "banana",
            "label": "Nano Banana"
          },
          {
            "id": "seedream",
            "label": "Seedream"
          },
          {
            "id": "sd-video",
            "label": "SD 视频"
          }
        ]
      }
    ]
  },
  {
    "id": "sd-video",
    "label": "SD 视频",
    "title": "SD 2.0 / 2.5 视频",
    "lead": "SD 2.0与2.5的型号选择、分辨率、参考素材限制，以及异步任务查询和视频下载。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "1. 基础信息与型号选择"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "说明"
        ],
        "rows": [
          [
            "API Base URL",
            "`https://please.dontblockme.men`"
          ],
          [
            "创建视频",
            "`POST /v1/videos`"
          ],
          [
            "查询视频",
            "`GET /v1/videos/{id}`"
          ],
          [
            "下载视频",
            "`GET /v1/videos/{id}/content`"
          ],
          [
            "主要接入模式",
            "异步：创建 → 保存id → 查询 → completed → 下载"
          ],
          [
            "模型权限",
            "以当前Key在`GET /v1/models`返回的名称为准"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "分辨率",
          "参考图片",
          "参考音频",
          "参考视频"
        ],
        "rows": [
          [
            "`sd2.0-720p`",
            "`720p`",
            "最多9张",
            "最多3条",
            "不支持"
          ],
          [
            "`sd2.5-480p`",
            "`480p`",
            "最多30张",
            "最多10条",
            "最多10条"
          ],
          [
            "`sd2.5-720p`",
            "`720p`",
            "最多30张",
            "最多10条",
            "不支持"
          ],
          [
            "`sd2.5-1080p`",
            "`1080p`",
            "最多30张",
            "最多10条",
            "最多10条"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "四个型号共用本页的创建、查询和下载接口，resolution填写表中对应值。SD2.0使用sd2.0-720p，SD2.5的名称以sd2.5-开头，均不在sd与版本号之间增加连字符。"
      },
      {
        "type": "note",
        "value": "素材上限按每次请求、每种类型分别计算。sd2.0-720p最多9张图片、3条音频；sd2.5-720p最多30张图片、10条音频。两个720p型号均不支持参考视频，请省略video_urls。"
      },
      {
        "type": "heading",
        "value": "2. 鉴权与请求格式"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"<API_KEY>\""
      },
      {
        "type": "code",
        "lang": "http",
        "value": "Authorization: Bearer <API_KEY>\nContent-Type: application/json"
      },
      {
        "type": "paragraph",
        "value": "创建使用JSON，查询和下载只需要Authorization。cURL示例采用Bash换行语法；PowerShell请使用curl.exe并调整变量及换行。Key放在服务端或本机环境变量中，不写入公开网页。"
      },
      {
        "type": "heading",
        "value": "3. 请求参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "必填",
          "规则"
        ],
        "rows": [
          [
            "`model`",
            "string",
            "是",
            "使用上表中的完整模型名称"
          ],
          [
            "`prompt`",
            "string",
            "是",
            "描述主体、动作、场景、运镜；引用素材时使用`@Image1`、`@Video1`、`@Audio1`"
          ],
          [
            "`duration`",
            "integer",
            "否",
            "输出视频时长，单位秒，填写整数；示例使用6秒，不混传seconds"
          ],
          [
            "`resolution`",
            "string",
            "否",
            "与model对应：sd2.0-720p与sd2.5-720p填写720p；sd2.5-480p填写480p；sd2.5-1080p填写1080p"
          ],
          [
            "`aspect_ratio`",
            "string",
            "否",
            "默认`16:9`；支持`21:9`、`16:9`、`3:2`、`4:3`、`1:1`、`3:4`、`2:3`、`9:16`"
          ],
          [
            "`image_urls`",
            "string[]",
            "否",
            "图片HTTPS直链数组。sd2.0-720p最多9张；sd2.5-480p、sd2.5-720p与sd2.5-1080p均最多30张。从@Image1起编号"
          ],
          [
            "`video_urls`",
            "string[]",
            "否",
            "仅sd2.5-480p和sd2.5-1080p支持，最多10条；720p型号不支持。按数组顺序从@Video1起编号"
          ],
          [
            "`audio_urls`",
            "string[]",
            "否",
            "音频HTTPS直链数组。sd2.0-720p最多3条；sd2.5-480p、sd2.5-720p与sd2.5-1080p均最多10条。从@Audio1起编号"
          ],
          [
            "`first_frame`",
            "string",
            "否",
            "仅SD2.5：首帧图片HTTPS直链，必须与last_frame一起提供"
          ],
          [
            "`last_frame`",
            "string",
            "否",
            "仅SD2.5：尾帧图片HTTPS直链；首尾帧模式与三类refs数组二选一"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "纯文生视频省略所有素材字段。多素材模式使用image_urls、video_urls、audio_urls；SD2.5首尾帧使用first_frame和last_frame，与素材数组二选一。网关负责字段映射，客户端无需使用上游私有字段，也不需要把视频或音频转成Base64。"
      },
      {
        "type": "heading",
        "value": "4. 文生视频"
      },
      {
        "type": "paragraph",
        "value": "SD2.0使用以下请求。切换为sd2.5-720p时只需替换model，resolution仍为720p，后续任务查询和下载流程相同。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"sd2.0-720p\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍\",\n  \"duration\": 6,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"sd2.5-720p\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍\",\n  \"duration\": 6,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "paragraph",
        "value": "视频接口本身异步执行：不加Prefer请求头时，创建返回视频任务id；保存id后查询到completed再下载。创建响应不是MP4文件，不使用stream=true等待视频字节。"
      },
      {
        "type": "heading",
        "value": "5. 图片参考与首尾帧"
      },
      {
        "type": "paragraph",
        "value": "sd2.0-720p与sd2.5-720p均支持图片参考：将图片放入image_urls，并用@Image1说明用途。多图时按数组顺序引用@Image1、@Image2，以此类推；编号区分大小写。sd2.0-720p最多9张图片，sd2.5-720p最多30张图片。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"sd2.5-720p\",\n  \"prompt\": \"保持@Image1中商品的外观，镜头缓慢环绕，光线柔和，背景简洁\",\n  \"duration\": 6,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://example.com/product.jpg\"\n  ]\n}'"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"sd2.5-1080p\",\n  \"prompt\": \"人物外观参考@Image1，服装参考@Image2，人物自然转身展示服装，镜头稳定\",\n  \"duration\": 10,\n  \"resolution\": \"1080p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://example.com/person.jpg\",\n    \"https://example.com/clothes.jpg\"\n  ]\n}"
      },
      {
        "type": "paragraph",
        "value": "first_frame与last_frame必须同时填写，并省略image_urls、video_urls和audio_urls。以下使用sd2.5-720p，分辨率填写720p。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"sd2.5-720p\",\n  \"prompt\": \"从首帧平滑过渡到尾帧，保持主体外观和场景一致，动作连贯自然\",\n  \"duration\": 10,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\",\n  \"first_frame\": \"https://example.com/first.jpg\",\n  \"last_frame\": \"https://example.com/last.jpg\"\n}"
      },
      {
        "type": "heading",
        "value": "6. 视频、音频参考与素材要求"
      },
      {
        "type": "paragraph",
        "value": "视频参考仅限sd2.5-480p和sd2.5-1080p。下面使用480p型号，同时传入图片、视频和音频参考。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  --data '{\n  \"model\": \"sd2.5-480p\",\n  \"prompt\": \"保持@Image1的主体外观，参考@Video1的动作，配合@Audio1的节奏，镜头平稳\",\n  \"duration\": 6,\n  \"resolution\": \"480p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://example.com/subject.jpg\"\n  ],\n  \"video_urls\": [\n    \"https://example.com/motion.mp4\"\n  ],\n  \"audio_urls\": [\n    \"https://example.com/rhythm.wav\"\n  ]\n}'"
      },
      {
        "type": "paragraph",
        "value": "720p型号支持图片和音频参考，不传video_urls："
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"model\": \"sd2.5-720p\",\n  \"prompt\": \"保持@Image1的主体外观，让动作配合@Audio1的节奏\",\n  \"duration\": 6,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\",\n  \"image_urls\": [\n    \"https://example.com/person.jpg\"\n  ],\n  \"audio_urls\": [\n    \"https://example.com/rhythm.wav\"\n  ]\n}"
      },
      {
        "type": "list",
        "items": [
          "三类素材各自从1编号，例如image_urls[0]对应@Image1，video_urls[0]对应@Video1，audio_urls[0]对应@Audio1；提示词只引用实际传入的素材。",
          "使用直接返回媒体文件的HTTPS链接，不使用网页、网盘预览页、本机路径或需要登录的地址。生成服务不会携带你的Cookie或额外请求头。",
          "签名链接需在排队和生成期间保持有效；Content-Type应与文件实际格式一致。",
          "先上传图片、视频或音频再传直链，避免内联大段Base64增加请求体积、上传耗时和服务器内存占用。",
          "只传本次需要的素材。长音频或大体积视频优先截取相关片段；duration表示输出视频时长，不是参考音频长度。",
          "audio_urls是音频参考，不是强制生成音轨的开关；不要凭该字段推断输出必定带音频。",
          "素材数量在提交前按型号分别校验，首尾帧模式不与参考数组混传。"
        ]
      },
      {
        "type": "heading",
        "value": "7. 创建任务响应"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"sd2.5-720p\",\n  \"status\": \"queued\",\n  \"progress\": 0,\n  \"created_at\": 1790900000\n}"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "说明"
        ],
        "rows": [
          [
            "`id`",
            "string",
            "本站视频任务编号，后续查询与下载使用此值"
          ],
          [
            "`task_id`",
            "string，可选",
            "旧客户端可能收到的兼容字段；优先使用id，不要求两者同时存在"
          ],
          [
            "`object`",
            "string",
            "`video`"
          ],
          [
            "`model`",
            "string",
            "本次请求的公开模型名称"
          ],
          [
            "`status`",
            "string",
            "通常为queued；具体含义见状态表"
          ],
          [
            "`progress`",
            "integer",
            "0—100的进度提示；不是带百分号的字符串，也不代表剩余秒数"
          ],
          [
            "`created_at`",
            "integer",
            "Unix时间戳，单位秒"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "创建成功接收2xx响应（通常200或202），同时校验JSON中的id。HTTP成功只代表任务已受理，不代表视频生成完成。不要把上游私有编号替代本站id。"
      },
      {
        "type": "heading",
        "value": "8. 查询任务与状态机"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/videos/task_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"sd2.5-720p\",\n  \"status\": \"in_progress\",\n  \"progress\": 45,\n  \"created_at\": 1790900000\n}"
      },
      {
        "type": "code",
        "lang": "text",
        "value": "queued → in_progress → completed → GET /v1/videos/{id}/content\n                    ↘ failed    → 记录错误并停止查询"
      },
      {
        "type": "table",
        "headers": [
          "status",
          "含义",
          "客户端动作"
        ],
        "rows": [
          [
            "`queued`",
            "等待处理",
            "继续查询相同id"
          ],
          [
            "`in_progress`",
            "正在提交或生成",
            "继续查询，不重新创建"
          ],
          [
            "`completed`",
            "生成成功",
            "开始下载文件"
          ],
          [
            "`failed`",
            "生成失败",
            "保存响应和错误信息，停止查询"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "建议从15秒一次开始查询；遇到Retry-After遵循服务端提示，429和临时5xx只重试查询并退避。设置本地等待期限，超时后保留id继续查询，而不是再次POST创建。未知状态不要当成成功。"
      },
      {
        "type": "heading",
        "value": "9. 成功响应与结果字段"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"sd2.5-720p\",\n  \"status\": \"completed\",\n  \"progress\": 100,\n  \"created_at\": 1790900000,\n  \"completed_at\": 1790900120\n}"
      },
      {
        "type": "paragraph",
        "value": "completed_at是生成完成时间，单位秒。不同协议可能附带video_urls、metadata或其他扩展字段，但这些不是本站客户端必须依赖的契约。本站不会保证暴露上游私有下载地址；统一使用内容下载接口。"
      },
      {
        "type": "heading",
        "value": "10. 下载与保存视频"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 300 \\\n  'https://please.dontblockme.men/v1/videos/task_example/content' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.mp4"
      },
      {
        "type": "list",
        "items": [
          "只在status为completed后下载，响应正文是媒体字节，不是JSON。",
          "内容接口需要Bearer鉴权，直接粘贴到浏览器地址栏可能返回401。",
          "保存到自己的存储；任务记录存在不代表归档文件永久有效，过期文件可能返回410。",
          "程序采用流式写文件，避免把完整视频一次性加载到内存。",
          "下载网络错误可重试同一文件；不要因此重新生成视频。"
        ]
      },
      {
        "type": "heading",
        "value": "11. 失败响应与排错"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"task_example\",\n  \"object\": \"video\",\n  \"model\": \"sd2.5-720p\",\n  \"status\": \"failed\",\n  \"progress\": 100,\n  \"created_at\": 1790900000,\n  \"error\": {\n    \"message\": \"参考素材读取失败，请检查媒体地址\",\n    \"code\": \"generation_failed\"\n  }\n}"
      },
      {
        "type": "paragraph",
        "value": "此处为失败结构示意，具体message和code由实际错误决定，error字段可能省略。始终以status判断是否失败；HTTP 200的查询响应也可能携带failed。"
      },
      {
        "type": "table",
        "headers": [
          "HTTP状态",
          "可能原因",
          "处理方式"
        ],
        "rows": [
          [
            "400",
            "参数类型、时长、分辨率或素材地址不匹配",
            "核对模型表与字段，保存错误正文"
          ],
          [
            "401",
            "Key无效或缺失",
            "检查Bearer请求头及当前Key"
          ],
          [
            "402 / 403",
            "余额、额度或权限限制",
            "结合error中的原因检查账户余额、令牌与分组权限"
          ],
          [
            "404",
            "路径错误、任务不存在或不属于当前账户",
            "核对完整id和创建时使用的账户"
          ],
          [
            "400 / 409",
            "视频未完成就请求下载",
            "先查询到completed；不同下载入口的状态码可能不同"
          ],
          [
            "410",
            "归档结果已过期",
            "及时保存文件；先确认结果确实过期再决定是否重新生成"
          ],
          [
            "429",
            "限流或并发过高",
            "尊重Retry-After；降低查询频率"
          ],
          [
            "502 / 503 / 504",
            "上游、代理或网络临时异常",
            "查询请求可退避重试；创建超时先确认原任务是否已经受理"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "12. 网关异步模式：与Videos任务的区别"
      },
      {
        "type": "table",
        "headers": [
          "模式",
          "创建方式",
          "编号与查询",
          "完成状态"
        ],
        "rows": [
          [
            "Videos任务（本页默认）",
            "不添加异步请求头",
            "`task_...` → `/v1/videos/{id}`",
            "`completed`"
          ],
          [
            "网关异步任务",
            "添加`Prefer: respond-async`或`?async=true`",
            "`async_...` → 返回的`poll_url`，通常`/v1/tasks/{id}`",
            "`succeeded`"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "两种方式均能异步完成生成，但返回对象与状态机不同。按创建响应选择一套流程，不把completed与succeeded、task_编号与async_编号混用。下面给出网关模式的完整请求和结果结构。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/videos' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"sd2.5-720p\",\n  \"prompt\": \"晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍，镜头自然推进\",\n  \"duration\": 6,\n  \"resolution\": \"720p\",\n  \"aspect_ratio\": \"16:9\"\n}'"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"video_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"sd2.5-720p\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/videos\"\n}"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "table",
        "headers": [
          "网关status",
          "含义"
        ],
        "rows": [
          [
            "pending / processing / waiting",
            "排队、处理或等待视频任务完成，继续查询"
          ],
          [
            "succeeded",
            "已完成，检查result_expired并读取media"
          ],
          [
            "failed / cancelled",
            "终止查询，读取error.message"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"id\": \"task_example\",\n    \"object\": \"video\",\n    \"model\": \"sd2.5-720p\",\n    \"status\": \"completed\",\n    \"progress\": 100,\n    \"created_at\": 1790900000,\n    \"completed_at\": 1790900120\n  },\n  \"media\": [\n    {\n      \"url\": \"/v1/tasks/async_example/media/0\",\n      \"kind\": \"video\",\n      \"content_type\": \"video/mp4\"\n    }\n  ]\n}"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 300 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" --output result.mp4"
      },
      {
        "type": "paragraph",
        "value": "media[].url需要鉴权；preview_url若存在是临时展示地址。result_expired为true时停止下载。第13、14节示例使用Videos任务接口，第16节示例使用本节的网关任务接口，任选一种模式接入。"
      },
      {
        "type": "heading",
        "value": "13. Node.js完整示例"
      },
      {
        "type": "paragraph",
        "value": "完整示例默认使用sd2.5-720p。设置TOKENFREE_SD_MODEL为sd2.0-720p即可使用SD2.0，也可填写表中的其他SD2.5型号，示例会自动匹配分辨率。Node.js、Python与网关示例使用同一变量。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_SD_MODEL=\"sd2.0-720p\"\n# 也可填写sd2.5-480p、sd2.5-720p或sd2.5-1080p"
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_SD_MODEL = \"sd2.0-720p\""
      },
      {
        "type": "paragraph",
        "value": "Node.js 22或更新版本，无额外依赖。保存为sd-video.mjs，设置TOKENFREE_API_KEY，在新目录执行。此示例使用默认Videos任务，不加Prefer请求头。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node sd-video.mjs\n# 恢复同一任务\nnode sd-video.mjs video-task.json"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\nconst taskFile = process.argv[2] || 'video-task.json';\n// 型号与分辨率绑定，只修改型号即可切换对应清晰度。\nconst model = process.env.TOKENFREE_SD_MODEL || 'sd2.5-720p';\nconst resolutions = {\n  'sd2.0-720p': '720p',\n  'sd2.5-480p': '480p',\n  'sd2.5-720p': '720p',\n  'sd2.5-1080p': '1080p'\n};\nif (!Object.hasOwn(resolutions, model)) throw new Error('未知SD模型：' + model);\nconst request = {\n  model,\n  prompt: '晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍',\n  duration: 6, resolution: resolutions[model], aspect_ratio: '16:9'\n  // 图生视频时在提示词中引用@Image1，并增加 image_urls: ['https://你的域名/参考图.jpg']。\n};\n\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction retryAfter(response, fallback = 15000) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return fallback;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms >= 0 ? Math.max(ms, 1000) : fallback;\n}\n\nlet submitted;\nlet interval = 15000;\nif (process.argv[2]) {\n  // 恢复时只读取已保存的编号，不再次创建收费任务。\n  submitted = JSON.parse(await readFile(taskFile, 'utf8'));\n} else {\n  if (existsSync(taskFile)) throw new Error('已有video-task.json，请传入此文件恢复，或在新目录创建新任务');\n  // 创建请求只发送一次；连接超时也不自动重发。\n  const response = await fetch(ORIGIN + '/v1/videos', {\n    method: 'POST', headers: { ...headers, 'Content-Type': 'application/json' },\n    body: JSON.stringify(request), redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (!submitted.id && !submitted.task_id) throw new Error('响应缺少视频任务id');\n  console.log('请保留任务编号：', submitted.id || submitted.task_id);\n  await writeFile(taskFile, JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  interval = retryAfter(response);\n}\nconst id = submitted.id || submitted.task_id;\nif (typeof id !== 'string' || !id || id.startsWith('async_')) {\n  throw new Error('需要Videos任务id；async_编号请使用网关任务示例');\n}\nconst endpoint = ORIGIN + '/v1/videos/' + encodeURIComponent(id);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() + interval < deadline) {\n  await delay(interval);\n  let response;\n  try {\n    response = await fetch(endpoint, { headers, redirect: 'error', signal: AbortSignal.timeout(30000) });\n  } catch {\n    // 网络故障只重试查询，不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (response.status === 429 || response.status >= 500) {\n    interval = Math.max(retryAfter(response), Math.min(interval * 2, 30000));\n    await response.body?.cancel();\n    continue;\n  }\n  task = await readJson(response);\n  interval = retryAfter(response);\n  console.log('任务状态：', task.status, '进度：', task.progress ?? '未提供');\n  if (task.status === 'completed') break;\n  if (task.status === 'failed') throw new Error(task.error?.message || '视频生成失败：' + id);\n  if (!['queued', 'in_progress'].includes(task.status)) throw new Error('未知视频状态：' + task.status);\n}\nif (task?.status !== 'completed') throw new Error('本地等待结束；传入任务文件继续查询，服务端任务不会因此取消');\nawait writeFile('video-result.json', JSON.stringify(task, null, 2));\n\n// 流式下载；完成前使用临时扩展名，避免把不完整视频误认为成品。\nconst response = await fetch(endpoint + '/content', {\n  headers, redirect: 'error', signal: AbortSignal.timeout(300000)\n});\nif (!response.ok) throw new Error('视频下载失败，HTTP ' + response.status + '；可以用原任务文件重试');\nif (!response.body) throw new Error('下载响应缺少文件内容');\nconst mime = (response.headers.get('Content-Type') || '').split(';')[0];\nif (mime && !mime.startsWith('video/') && mime !== 'application/octet-stream') {\n  throw new Error('下载返回的不是视频：' + mime);\n}\ntry {\n  await pipeline(Readable.fromWeb(response.body), createWriteStream('result.mp4.part'));\n  await rename('result.mp4.part', 'result.mp4');\n} catch (error) {\n  await rm('result.mp4.part', { force: true });\n  throw error;\n}\nconsole.log('已保存 result.mp4');\n"
      },
      {
        "type": "heading",
        "value": "14. Python完整示例"
      },
      {
        "type": "paragraph",
        "value": "Python 3.10或更新版本，仅使用标准库，无需pip安装。保存为sd-video.py，与Node.js示例任选一个运行，不要为同一目标同时运行两个创建示例。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "python sd-video.py\n# 恢复同一任务\npython sd-video.py video-task.json"
      },
      {
        "type": "code",
        "lang": "python",
        "value": "import json\nimport os\nimport sys\nimport time\nimport urllib.error\nimport urllib.parse\nimport urllib.request\nfrom datetime import datetime, timezone\nfrom email.utils import parsedate_to_datetime\nfrom pathlib import Path\n\nORIGIN = 'https://please.dontblockme.men'\nKEY = os.environ.get('TOKENFREE_API_KEY')\nif not KEY:\n    raise SystemExit('请设置 TOKENFREE_API_KEY')\nTASK_FILE = Path(sys.argv[1] if len(sys.argv) > 1 else 'video-task.json')\nMODEL = os.environ.get('TOKENFREE_SD_MODEL') or 'sd2.5-720p'\n# 型号与分辨率绑定，避免高清型号误用示例的默认清晰度。\nRESOLUTIONS = {\n    'sd2.0-720p': '720p',\n    'sd2.5-480p': '480p',\n    'sd2.5-720p': '720p',\n    'sd2.5-1080p': '1080p',\n}\nif MODEL not in RESOLUTIONS:\n    raise SystemExit('未知SD模型：' + MODEL)\nBODY = {\n    'model': MODEL,\n    'prompt': '晨光中的海边公路，一辆蓝色轿车平稳行驶，低机位跟拍',\n    'duration': 6, 'resolution': RESOLUTIONS[MODEL], 'aspect_ratio': '16:9',\n    # 图生视频时在提示词中引用@Image1，并增加 image_urls: ['https://你的域名/参考图.jpg']。\n}\n\n\nclass NoRedirect(urllib.request.HTTPRedirectHandler):\n    # 不向跳转后的其他地址转发鉴权信息。\n    def redirect_request(self, req, fp, code, msg, headers, newurl):\n        return None\n\n\nOPENER = urllib.request.build_opener(NoRedirect())\n\n\ndef open_request(path, data=None, timeout=30):\n    headers = {'Authorization': 'Bearer ' + KEY}\n    if data is not None:\n        headers['Content-Type'] = 'application/json'\n    request = urllib.request.Request(ORIGIN + path, data=data, headers=headers)\n    return OPENER.open(request, timeout=timeout)\n\n\ndef retry_after(headers, fallback=15):\n    value = headers.get('Retry-After')\n    if not value:\n        return fallback\n    try:\n        return max(float(value), 1)\n    except ValueError:\n        try:\n            return max((parsedate_to_datetime(value) - datetime.now(timezone.utc)).total_seconds(), 1)\n        except (TypeError, ValueError, OverflowError):\n            return fallback\n\n\ndef main():\n    interval = 15\n    if len(sys.argv) > 1:\n        # 传入保存的任务文件恢复查询，不创建新任务。\n        submitted = json.loads(TASK_FILE.read_text(encoding='utf-8'))\n    else:\n        if TASK_FILE.exists():\n            raise RuntimeError('已有video-task.json，请传入此文件恢复，或在新目录创建新任务')\n        data = json.dumps(BODY, ensure_ascii=False).encode('utf-8')\n        # 创建只尝试一次；超时不等于服务端没有受理。\n        with open_request('/v1/videos', data, timeout=120) as response:\n            submitted = json.load(response)\n            interval = retry_after(response.headers)\n        task_id = submitted.get('id') or submitted.get('task_id')\n        if not task_id:\n            raise RuntimeError('响应缺少视频任务id')\n        print('请保留任务编号：', task_id)\n        with TASK_FILE.open('x', encoding='utf-8') as file:\n            json.dump(submitted, file, ensure_ascii=False, indent=2)\n    task_id = submitted.get('id') or submitted.get('task_id')\n    if not isinstance(task_id, str) or not task_id or task_id.startswith('async_'):\n        raise RuntimeError('需要Videos任务id；async_编号请使用网关任务示例')\n    endpoint = '/v1/videos/' + urllib.parse.quote(task_id, safe='')\n    deadline = time.monotonic() + 30 * 60\n    task = {}\n    while time.monotonic() + interval < deadline:\n        time.sleep(interval)\n        try:\n            with open_request(endpoint) as response:\n                task = json.load(response)\n                interval = retry_after(response.headers)\n        except urllib.error.HTTPError as error:\n            if error.code == 429 or error.code >= 500:\n                interval = max(retry_after(error.headers), min(interval * 2, 30))\n                error.close()\n                continue\n            raise\n        except (urllib.error.URLError, TimeoutError, ConnectionError):\n            interval = min(interval * 2, 30)\n            continue\n        print('任务状态：', task.get('status'), '进度：', task.get('progress', '未提供'))\n        if task.get('status') == 'completed':\n            break\n        if task.get('status') == 'failed':\n            raise RuntimeError((task.get('error') or {}).get('message') or '视频生成失败：' + task_id)\n        if task.get('status') not in ('queued', 'in_progress'):\n            raise RuntimeError('未知视频状态：' + str(task.get('status')))\n    if task.get('status') != 'completed':\n        raise RuntimeError('本地等待结束；传入任务文件继续查询，服务端任务不会因此取消')\n    Path('video-result.json').write_text(json.dumps(task, ensure_ascii=False, indent=2), encoding='utf-8')\n    partial = Path('result.mp4.part')\n    try:\n        # 固定大小分块下载，不把整个视频加载到内存。\n        with open_request(endpoint + '/content', timeout=300) as response:\n            mime = response.headers.get('Content-Type', '').split(';')[0]\n            if mime and not mime.startswith('video/') and mime != 'application/octet-stream':\n                raise RuntimeError('下载返回的不是视频：' + mime)\n            with partial.open('wb') as file:\n                while True:\n                    chunk = response.read(1024 * 1024)\n                    if not chunk:\n                        break\n                    file.write(chunk)\n        partial.replace('result.mp4')\n    finally:\n        partial.unlink(missing_ok=True)\n    print('已保存 result.mp4')\n\n\nif __name__ == '__main__':\n    try:\n        main()\n    except urllib.error.HTTPError as error:\n        # 输出有限长度的错误正文，避免把大型非JSON响应写满终端。\n        print('HTTP', error.code, error.read(4096).decode('utf-8', errors='replace'), file=sys.stderr)\n        error.close()\n        sys.exit(1)\n    except Exception as error:\n        print(str(error), file=sys.stderr)\n        sys.exit(1)\n"
      },
      {
        "type": "note",
        "value": "两个示例立即保存video-task.json，等待completed后流式写入result.mp4；失败停止，429和临时查询故障退避。30分钟是客户端等待期限，不取消服务端任务。下载失败可用原任务文件重试，不重新生成。Python的timeout用于阻塞网络操作，Node.js的AbortSignal用于请求超时。"
      },
      {
        "type": "heading",
        "value": "15. 重试、计费与接入检查"
      },
      {
        "type": "list",
        "items": [
          "生成费用按平台实际价格、时长和账户分组结算，不把文档示例中的时长当作固定价格。",
          "POST创建不做自动重试。连接断开、504或本地超时，不证明服务端没有接收任务。",
          "收到id后立即落盘，后续继续查询这个id。查询和下载的重试不应再触发创建。",
          "只有确认原任务失败并处理原因后，才考虑创建新任务。不要承诺所有失败都会按同一种方式退款。",
          "向售后提供模型、任务id、请求时间、HTTP状态和去掉Key的参数，避免发送完整API Key。"
        ]
      },
      {
        "type": "heading",
        "value": "16. 网关模式完整示例"
      },
      {
        "type": "paragraph",
        "value": "若希望图片与视频共用网关任务查询方式，使用以下Node.js示例。保存为sd-video-gateway.mjs，在新目录设置TOKENFREE_API_KEY后运行。此示例添加Prefer: respond-async，并通过返回的poll_url查询到succeeded；不要把task.json与Videos模式的video-task.json混用。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node sd-video-gateway.mjs\n# 恢复同一网关任务\nnode sd-video-gateway.mjs task.json"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n// 型号的后缀就是对应分辨率，网关示例与标准视频示例采用同一组型号。\nconst model = process.env.TOKENFREE_SD_MODEL || 'sd2.5-720p';\nconst resolutions = {\n  'sd2.0-720p': '720p',\n  'sd2.5-480p': '480p',\n  'sd2.5-720p': '720p',\n  'sd2.5-1080p': '1080p'\n};\nif (!Object.hasOwn(resolutions, model)) throw new Error('未知SD模型：' + model);\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 15000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 15000;\n}\n\nlet submitted;\nlet interval = 15000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/videos', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model, prompt: '海边公路上一辆蓝色轿车平稳行驶', duration: 6, resolution: resolutions[model], aspect_ratio: '16:9' }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node sd-video-gateway.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "17. 常见问题"
      },
      {
        "type": "list",
        "items": [
          "参考素材没有生效：核对refs字段名称、HTTPS文件可访问性、素材数组顺序及提示词中的@Image / @Video / @Audio编号。",
          "清晰度不匹配：sd2.5-480p填写480p，sd2.5-1080p填写1080p，sd2.0-720p与sd2.5-720p填写720p。",
          "参考视频被拒绝：sd2.0-720p与sd2.5-720p不支持video_urls；需要视频参考时选择sd2.5-480p或sd2.5-1080p。",
          "首尾帧请求报错：使用SD2.5型号，同时提供first_frame和last_frame，移除所有refs数组。",
          "提交超时或408 / 413：检查请求体积和素材是否可读取，减少不必要的内联数据；先确认原任务是否受理，避免重复创建。",
          "下载返回401：内容接口需要Bearer鉴权，浏览器地址栏不会自动附带Key。",
          "查询HTTP 200但生成失败：检查status与error.message，HTTP状态只代表查询是否成功。",
          "向售后提供模型名称、任务id、请求时间、HTTP状态和去掉Key的参数。"
        ]
      },
      {
        "type": "links",
        "items": [
          {
            "id": "image2",
            "label": "GPT Image"
          },
          {
            "id": "banana",
            "label": "Nano Banana"
          },
          {
            "id": "seedream",
            "label": "Seedream"
          },
          {
            "id": "video",
            "label": "Wan 视频"
          }
        ]
      }
    ]
  },
  {
    "id": "image-1k",
    "label": "1k-image",
    "title": "1k-image · GPT Image",
    "lead": "本分组的模型名称、参数能力、比例尺寸、URL或本地参考图、图片编辑与任务处理。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "文生图",
            "POST /v1/images/generations"
          ],
          [
            "图生图 / 编辑",
            "POST /v1/images/edits"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "分组",
            "1k-image"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"YOUR_API_KEY\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"YOUR_API_KEY\""
      },
      {
        "type": "paragraph",
        "value": "使用获准访问本分组的Key。同名模型可以出现在不同分组中，仅修改model不会切换Key的分组选路。普通Key使用自身分组；生图和视频Key按获授权的分组顺序匹配，需要固定某条线路时使用对应分组的普通Key。"
      },
      {
        "type": "heading",
        "value": "选择模型"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "尺寸档位",
          "quality",
          "background",
          "mask局部编辑",
          "线路"
        ],
        "rows": [
          [
            "gpt-image-2",
            "仅1K",
            "auto / low / medium / high",
            "不提供，省略",
            "不开放",
            "基础1K"
          ],
          [
            "gpt-image-2.5-flare",
            "仅1K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "不开放",
            "基础1K"
          ],
          [
            "gpt-image-2.5-sunburst",
            "仅1K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "不开放",
            "基础1K"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求与说明"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "完整填写本页列出的模型名，保留AZ-、官-或规格后缀"
          ],
          [
            "prompt",
            "string，必填",
            "描述主体、构图、风格及画中文字，不能为空"
          ],
          [
            "size",
            "string，建议显式填写",
            "具体像素宽x高，按本页型号能力和尺寸表选择"
          ],
          [
            "quality",
            "string，可选",
            "按型号表选择；quality改变细节偏好，不改变size的格式"
          ],
          [
            "n",
            "integer，建议1",
            "建议每次1张，多张独立结果分别创建并保存任务编号"
          ],
          [
            "background",
            "string，可选",
            "本分组不提供背景参数，省略"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json，控制返回结果，与输入参考图URL无关"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；透明背景选择png或webp"
          ],
          [
            "output_compression",
            "integer，可选",
            "JPEG/WebP压缩设置0–100；PNG省略"
          ],
          [
            "moderation",
            "string，可选",
            "auto为默认审核，low为较宽松级别，不代表关闭内容审核"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "比例、尺寸与像素限制"
      },
      {
        "type": "paragraph",
        "value": "本分组只使用1K列；不要因为模型名称与Adobe分组相同，就发送2K或4K尺寸。size填写具体的宽x高，使用英文小写x。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024"
          ],
          [
            "3:2",
            "1536x1024"
          ],
          [
            "2:3",
            "1024x1536"
          ],
          [
            "16:9",
            "1280x720"
          ],
          [
            "9:16",
            "720x1280"
          ],
          [
            "4:3",
            "1024x768"
          ],
          [
            "3:4",
            "768x1024"
          ],
          [
            "21:9",
            "1280x544"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "约束",
          "要求"
        ],
        "rows": [
          [
            "宽高对齐",
            "宽和高均为16的倍数"
          ],
          [
            "最长边",
            "不超过3840px"
          ],
          [
            "长宽比",
            "最长边 / 最短边不超过3"
          ],
          [
            "总像素",
            "655360至8294400像素；同时遵守型号的档位限制"
          ],
          [
            "自定义尺寸",
            "同时满足对齐、最长边、长宽比和总像素要求；读取实际结果确认最终尺寸"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图完整请求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"size\": \"1280x720\",\n  \"quality\": \"high\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "POST /v1/images/edits可使用本地文件或公开图片URL。单图JSON用image字符串；多图字段按下表的线路格式填写。不要把本机路径或网页预览地址当成图片URL。"
      },
      {
        "type": "table",
        "headers": [
          "输入方式",
          "Content-Type",
          "字段和限制"
        ],
        "rows": [
          [
            "本地文件",
            "multipart/form-data",
            "单图image；多图重复image[]，PNG、JPEG或WebP，最多16张参考图"
          ],
          [
            "单张URL",
            "application/json",
            "image为一张公开图片URL；与images二选一"
          ],
          [
            "多张URL",
            "application/json",
            "images为图片URL字符串数组，最多16项"
          ],
          [
            "mask",
            "multipart文件，可选",
            "本分组基础1K不开放遮罩，省略"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "编辑字段",
          "类型 / 要求",
          "填写方式"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "使用本页型号表中的完整名称，文生图和编辑的型号相同"
          ],
          [
            "prompt",
            "string，必填",
            "写清保留、替换或修改的内容，多图按顺序说明各图用途"
          ],
          [
            "size",
            "string，建议填写",
            "使用具体像素，遵守本页型号档位和尺寸限制"
          ],
          [
            "quality",
            "string，可选",
            "Adobe、AZ和官key按型号表选择；基础1K的整图编辑请求省略"
          ],
          [
            "n",
            "integer，建议1",
            "输出数量，不是参考图张数；Adobe支持1–4"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时传入；Adobe Image 2不发送"
          ],
          [
            "moderation",
            "string，可选",
            "基础1K、AZ和官key可选auto或low；Adobe不提供独立控制，省略"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json；与参考图片的输入方式无关"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；实际结果按Content-Type保存"
          ],
          [
            "mask",
            "文件，可选",
            "仅支持局部编辑的型号使用；透明区域表示修改范围"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "单张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"1280x720\",\n  \"n\": 1,\n  \"image\": \"https://example.com/product.png\"\n}'"
      },
      {
        "type": "heading",
        "value": "多张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"1280x720\",\n  \"n\": 1,\n  \"images\": [\n    \"https://example.com/product.png\",\n    \"https://example.com/style.jpg\"\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "本地图片上传"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=gpt-image-2.5-flare' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.jpg' \\\n  -F 'size=1280x720' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "multipart上传由HTTP客户端自动生成boundary，不手动填写Content-Type。URL必须直接返回图片文件且在排队、生成期间保持有效；不要使用image=@https://...。"
      },
      {
        "type": "heading",
        "value": "图片结果字段与格式"
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "类型",
          "处理方式"
        ],
        "rows": [
          [
            "data",
            "array",
            "逐项处理生成结果，不丢弃后续图片"
          ],
          [
            "data[].url",
            "string，可选",
            "及时下载保存，临时URL不是永久存储"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "Base64解码成图片文件，不把编码文本直接写入.png"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "修订提示词，缺少此字段不等于生成失败"
          ],
          [
            "size / quality / background",
            "可选",
            "返回时记录实际参数，不凭请求值推断图片最终宽高"
          ],
          [
            "Content-Type",
            "文件下载响应头",
            "按实际MIME确定格式，不只根据URL后缀判断"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "同步响应需要在data中找到url或b64_json才算拿到图片。异步任务的result保留相同结构，有media归档地址时直接鉴权下载，客户端不需要转换接口协议。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"gpt-image-2.5-flare\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: 'gpt-image-2.5-flare', prompt: '浅色背景上的一杯橘子汽水', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "模型不存在或无可用渠道",
            "核对Key所属分组、授权范围及完整模型名；AZ使用flare，不是flara"
          ],
          [
            "response_format被拒绝",
            "AZ与官key请求省略此字段，不改成b64_json后自动重新提交"
          ],
          [
            "透明背景参数被拒绝",
            "Adobe的GPT Image 2不提供此功能；仅在本页型号表标明支持时发送"
          ],
          [
            "上传成功但参考图未生效",
            "核对image、images的线路格式及数组顺序，在prompt里说明每张参考图用途"
          ],
          [
            "连接超时或客户端关闭",
            "先查任务日志或已保存的任务编号，不自动重新创建收费任务"
          ],
          [
            "查询成功但生成失败",
            "HTTP 200只代表查询成功，仍需检查status、error和result_expired"
          ],
          [
            "价格不同",
            "同名模型在不同分组中可能使用不同计费规则和倍率，以当前Key报价和实际账单为准"
          ]
        ]
      }
    ]
  },
  {
    "id": "image-az",
    "label": "az-image",
    "title": "az-image · GPT Image",
    "lead": "本分组的模型名称、参数能力、比例尺寸、URL或本地参考图、图片编辑与任务处理。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "文生图",
            "POST /v1/images/generations"
          ],
          [
            "图生图 / 编辑",
            "POST /v1/images/edits"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "分组",
            "az-image"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"YOUR_API_KEY\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"YOUR_API_KEY\""
      },
      {
        "type": "paragraph",
        "value": "使用获准访问本分组的Key。同名模型可以出现在不同分组中，仅修改model不会切换Key的分组选路。普通Key使用自身分组；生图和视频Key按获授权的分组顺序匹配，需要固定某条线路时使用对应分组的普通Key。"
      },
      {
        "type": "heading",
        "value": "选择模型"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "尺寸档位",
          "quality",
          "background",
          "mask局部编辑",
          "线路"
        ],
        "rows": [
          [
            "AZ-gpt-image-2",
            "1K、2K、4K",
            "auto / low / medium / high",
            "auto / opaque / transparent",
            "支持",
            "az-image"
          ],
          [
            "AZ-gpt-image-2.5-flare",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "az-image"
          ],
          [
            "AZ-gpt-image-2.5-sunburst",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "az-image"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求与说明"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "完整填写本页列出的模型名，保留AZ-、官-或规格后缀"
          ],
          [
            "prompt",
            "string，必填",
            "描述主体、构图、风格及画中文字，不能为空"
          ],
          [
            "size",
            "string，建议显式填写",
            "具体像素宽x高，按本页型号能力和尺寸表选择"
          ],
          [
            "quality",
            "string，可选",
            "按型号表选择；quality改变细节偏好，不改变size的格式"
          ],
          [
            "n",
            "integer，建议1",
            "建议每次1张，多张独立结果分别创建并保存任务编号"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时使用auto、opaque或transparent；Adobe的gpt-image-2及gpt-image-2-4k不发送此字段"
          ],
          [
            "response_format",
            "string，可选",
            "不发送，包括url和b64_json；此分组按官方请求协议提交，读取实际返回的data[].b64_json或任务media"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；透明背景选择png或webp"
          ],
          [
            "output_compression",
            "integer，可选",
            "JPEG/WebP压缩设置0–100；PNG省略"
          ],
          [
            "moderation",
            "string，可选",
            "auto为默认审核，low为较宽松级别，不代表关闭内容审核"
          ]
        ]
      },
      {
        "type": "note",
        "value": "AZ和官key分组不发送response_format，也不在失败后把它改成另一个值重新生成。output_format控制文件格式，不能代替response_format。"
      },
      {
        "type": "heading",
        "value": "比例、尺寸与像素限制"
      },
      {
        "type": "paragraph",
        "value": "size填写具体像素宽x高，不填写1K、4K或16:9。本页型号可使用下表的1K、2K、4K尺寸。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size",
          "2K：size",
          "4K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024",
            "2048x2048",
            "2880x2880"
          ],
          [
            "3:2",
            "1536x1024",
            "2160x1440",
            "3456x2304"
          ],
          [
            "2:3",
            "1024x1536",
            "1440x2160",
            "2304x3456"
          ],
          [
            "16:9",
            "1280x720",
            "2560x1440",
            "3840x2160"
          ],
          [
            "9:16",
            "720x1280",
            "1440x2560",
            "2160x3840"
          ],
          [
            "4:3",
            "1024x768",
            "2048x1536",
            "3200x2400"
          ],
          [
            "3:4",
            "768x1024",
            "1536x2048",
            "2400x3200"
          ],
          [
            "21:9",
            "1280x544",
            "2560x1088",
            "3840x1600"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "约束",
          "要求"
        ],
        "rows": [
          [
            "宽高对齐",
            "宽和高均为16的倍数"
          ],
          [
            "最长边",
            "不超过3840px"
          ],
          [
            "长宽比",
            "最长边 / 最短边不超过3"
          ],
          [
            "总像素",
            "655360至8294400像素；同时遵守型号的档位限制"
          ],
          [
            "4K方图",
            "2880x2880，不是4096x4096"
          ],
          [
            "自定义尺寸",
            "同时满足对齐、最长边、长宽比和总像素要求；读取实际结果确认最终尺寸"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图完整请求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"AZ-gpt-image-2.5-flare\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"size\": \"2560x1440\",\n  \"quality\": \"high\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "POST /v1/images/edits可使用本地文件或公开图片URL。单图JSON用image字符串；多图字段按下表的线路格式填写。不要把本机路径或网页预览地址当成图片URL。"
      },
      {
        "type": "table",
        "headers": [
          "输入方式",
          "Content-Type",
          "字段和限制"
        ],
        "rows": [
          [
            "本地文件",
            "multipart/form-data",
            "单图image；多图重复image[]，PNG、JPEG或WebP，最多16张参考图"
          ],
          [
            "单张URL",
            "application/json",
            "image为一张公开图片URL；与images二选一"
          ],
          [
            "多张URL",
            "application/json",
            "images为图片URL字符串数组，最多16项"
          ],
          [
            "mask",
            "multipart文件，可选",
            "支持局部编辑；透明区域为修改区域，与第一张参考图尺寸相同"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "编辑字段",
          "类型 / 要求",
          "填写方式"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "使用本页型号表中的完整名称，文生图和编辑的型号相同"
          ],
          [
            "prompt",
            "string，必填",
            "写清保留、替换或修改的内容，多图按顺序说明各图用途"
          ],
          [
            "size",
            "string，建议填写",
            "使用具体像素，遵守本页型号档位和尺寸限制"
          ],
          [
            "quality",
            "string，可选",
            "Adobe、AZ和官key按型号表选择；基础1K的整图编辑请求省略"
          ],
          [
            "n",
            "integer，建议1",
            "输出数量，不是参考图张数；Adobe支持1–4"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时传入；Adobe Image 2不发送"
          ],
          [
            "moderation",
            "string，可选",
            "基础1K、AZ和官key可选auto或low；Adobe不提供独立控制，省略"
          ],
          [
            "response_format",
            "string，可选",
            "本分组省略，不发送url或b64_json"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；实际结果按Content-Type保存"
          ],
          [
            "mask",
            "文件，可选",
            "仅支持局部编辑的型号使用；透明区域表示修改范围"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "单张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"AZ-gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"image\": \"https://example.com/product.png\"\n}'"
      },
      {
        "type": "heading",
        "value": "多张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"AZ-gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"images\": [\n    \"https://example.com/product.png\",\n    \"https://example.com/style.jpg\"\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "本地图片上传"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=AZ-gpt-image-2.5-flare' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.jpg' \\\n  -F 'size=2560x1440' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "multipart上传由HTTP客户端自动生成boundary，不手动填写Content-Type。URL必须直接返回图片文件且在排队、生成期间保持有效；不要使用image=@https://...。"
      },
      {
        "type": "heading",
        "value": "遮罩局部编辑"
      },
      {
        "type": "paragraph",
        "value": "mask与生成透明背景是不同功能。主图、遮罩各不超过50MiB；遮罩带透明通道，宽高与第一张源图相同，透明区域为修改区域。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=AZ-gpt-image-2.5-flare' \\\n  -F 'prompt=只替换遮罩透明区域中的背景，保留人物与衣服' \\\n  -F 'image=@original.png' \\\n  -F 'mask=@mask.png' \\\n  -F 'size=1024x1024' \\\n  -F 'quality=high' \\\n  -F 'n=1'"
      },
      {
        "type": "heading",
        "value": "透明背景"
      },
      {
        "type": "paragraph",
        "value": "使用background:transparent并选择PNG或WebP。opaque表示不透明，auto由模型决定。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"AZ-gpt-image-2.5-flare\",\n  \"prompt\": \"一个独立的橘子图标，透明背景，无投影\",\n  \"size\": \"1024x1024\",\n  \"quality\": \"high\",\n  \"background\": \"transparent\",\n  \"output_format\": \"png\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图片结果字段与格式"
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "类型",
          "处理方式"
        ],
        "rows": [
          [
            "data",
            "array",
            "逐项处理生成结果，不丢弃后续图片"
          ],
          [
            "data[].url",
            "string，可选",
            "及时下载保存，临时URL不是永久存储"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "Base64解码成图片文件，不把编码文本直接写入.png"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "修订提示词，缺少此字段不等于生成失败"
          ],
          [
            "size / quality / background",
            "可选",
            "返回时记录实际参数，不凭请求值推断图片最终宽高"
          ],
          [
            "Content-Type",
            "文件下载响应头",
            "按实际MIME确定格式，不只根据URL后缀判断"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "同步响应需要在data中找到url或b64_json才算拿到图片。异步任务的result保留相同结构，有media归档地址时直接鉴权下载，客户端不需要转换接口协议。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"AZ-gpt-image-2.5-flare\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: 'AZ-gpt-image-2.5-flare', prompt: '浅色背景上的一杯橘子汽水', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "模型不存在或无可用渠道",
            "核对Key所属分组、授权范围及完整模型名；AZ使用flare，不是flara"
          ],
          [
            "response_format被拒绝",
            "AZ与官key请求省略此字段，不改成b64_json后自动重新提交"
          ],
          [
            "透明背景参数被拒绝",
            "Adobe的GPT Image 2不提供此功能；仅在本页型号表标明支持时发送"
          ],
          [
            "上传成功但参考图未生效",
            "核对image、images的线路格式及数组顺序，在prompt里说明每张参考图用途"
          ],
          [
            "连接超时或客户端关闭",
            "先查任务日志或已保存的任务编号，不自动重新创建收费任务"
          ],
          [
            "查询成功但生成失败",
            "HTTP 200只代表查询成功，仍需检查status、error和result_expired"
          ],
          [
            "价格不同",
            "同名模型在不同分组中可能使用不同计费规则和倍率，以当前Key报价和实际账单为准"
          ]
        ]
      }
    ]
  },
  {
    "id": "image-adobe",
    "label": "adobe-image",
    "title": "adobe-image · GPT Image",
    "lead": "本分组的模型名称、参数能力、比例尺寸、URL或本地参考图、图片编辑与任务处理。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "文生图",
            "POST /v1/images/generations"
          ],
          [
            "图生图 / 编辑",
            "POST /v1/images/edits"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "分组",
            "adobe-image"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"YOUR_API_KEY\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"YOUR_API_KEY\""
      },
      {
        "type": "paragraph",
        "value": "使用获准访问本分组的Key。同名模型可以出现在不同分组中，仅修改model不会切换Key的分组选路。普通Key使用自身分组；生图和视频Key按获授权的分组顺序匹配，需要固定某条线路时使用对应分组的普通Key。"
      },
      {
        "type": "heading",
        "value": "选择模型"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "尺寸档位",
          "quality",
          "background",
          "mask局部编辑",
          "线路"
        ],
        "rows": [
          [
            "gpt-image-2",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "不提供，省略",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-flare",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ],
          [
            "gpt-image-2.5-sunburst",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "Adobe"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求与说明"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "完整填写本页列出的模型名，保留AZ-、官-或规格后缀"
          ],
          [
            "prompt",
            "string，必填",
            "描述主体、构图、风格及画中文字；Adobe型号非空且不超过32768字节（不是32768个汉字）"
          ],
          [
            "size",
            "string，建议显式填写",
            "具体像素宽x高，按本页型号能力和尺寸表选择"
          ],
          [
            "quality",
            "string，可选",
            "按型号表选择；quality改变细节偏好，不改变size的格式"
          ],
          [
            "n",
            "integer，建议1",
            "Adobe支持1–4，省略为1；基础和其他线路建议每次1张。输出数量与参考图张数不同"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时使用auto、opaque或transparent；Adobe的gpt-image-2及gpt-image-2-4k不发送此字段"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json，控制返回结果，与输入参考图URL无关"
          ],
          [
            "output_format",
            "string，可选",
            "Adobe兼容输入接受png、jpeg、webp；保存时以实际Content-Type为准，不能保证此参数使上游按指定格式编码"
          ],
          [
            "output_compression",
            "integer，可选",
            "Adobe线路不提供独立压缩控制，省略；其他线路JPEG/WebP可选0–100，PNG省略"
          ],
          [
            "moderation",
            "string，可选",
            "Adobe未提供独立审核级别控制，Adobe型号省略；基础1K型号仍可使用auto或low"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Adobe的画质与透明背景"
      },
      {
        "type": "table",
        "headers": [
          "模型",
          "画质映射",
          "透明背景"
        ],
        "rows": [
          [
            "GPT Image 2（含默认分组gpt-image-2-4k）",
            "low→1；medium/auto→3；high/max→5；xhigh兼容映射为max",
            "不支持background和transparent_background，请省略"
          ],
          [
            "GPT Image 2.5 Flare / Sunburst的Adobe型号",
            "low→1；medium/auto→3；high→5；max/xhigh→7",
            "支持auto、opaque、transparent"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "画质映射是Adobe线路的细节等级。不要把官key或AZ的扩展品质行为套到Adobe；也不要把透明PNG遮罩与生成透明背景混为一谈。Image 2可以使用编辑遮罩，但不提供生成透明背景参数。"
      },
      {
        "type": "heading",
        "value": "比例、尺寸与像素限制"
      },
      {
        "type": "paragraph",
        "value": "size填写具体像素宽x高，不填写1K、4K或16:9。本页型号可使用下表的1K、2K、4K尺寸。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size",
          "2K：size",
          "4K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024",
            "2048x2048",
            "2880x2880"
          ],
          [
            "3:2",
            "1536x1024",
            "2160x1440",
            "3456x2304"
          ],
          [
            "2:3",
            "1024x1536",
            "1440x2160",
            "2304x3456"
          ],
          [
            "16:9",
            "1280x720",
            "2560x1440",
            "3840x2160"
          ],
          [
            "9:16",
            "720x1280",
            "1440x2560",
            "2160x3840"
          ],
          [
            "4:3",
            "1024x768",
            "2048x1536",
            "3200x2400"
          ],
          [
            "3:4",
            "768x1024",
            "1536x2048",
            "2400x3200"
          ],
          [
            "21:9",
            "1280x544",
            "2560x1088",
            "3840x1600"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "约束",
          "要求"
        ],
        "rows": [
          [
            "宽高对齐",
            "宽和高均为16的倍数"
          ],
          [
            "最长边",
            "不超过3840px"
          ],
          [
            "长宽比",
            "最长边 / 最短边不超过3"
          ],
          [
            "总像素",
            "655360至8294400像素；同时遵守型号的档位限制"
          ],
          [
            "4K方图",
            "2880x2880，不是4096x4096"
          ],
          [
            "自定义尺寸",
            "同时满足对齐、最长边、长宽比和总像素要求；读取实际结果确认最终尺寸"
          ]
        ]
      },
      {
        "type": "note",
        "value": "Adobe高分辨率需要明确画幅，推荐直接传size，例如2048x2048或3840x2160。不要在2K、4K请求里使用auto来代替具体尺寸。"
      },
      {
        "type": "heading",
        "value": "文生图完整请求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"size\": \"2560x1440\",\n  \"quality\": \"high\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "POST /v1/images/edits可使用本地文件或公开图片URL。单图JSON用image字符串；多图字段按下表的线路格式填写。不要把本机路径或网页预览地址当成图片URL。"
      },
      {
        "type": "table",
        "headers": [
          "输入方式",
          "Content-Type",
          "字段和限制"
        ],
        "rows": [
          [
            "本地文件",
            "multipart/form-data",
            "单图image；多图重复image[]，PNG、JPEG或WebP，最多16张参考图"
          ],
          [
            "单张URL",
            "application/json",
            "image为一张公开图片URL；与images二选一"
          ],
          [
            "多张URL",
            "application/json",
            "Adobe使用images:[{\"image_url\":\"https://...\"}]，最多16项"
          ],
          [
            "mask",
            "multipart文件，可选",
            "支持局部编辑；透明区域为修改区域，与第一张参考图尺寸相同"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "编辑字段",
          "类型 / 要求",
          "填写方式"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "使用本页型号表中的完整名称，文生图和编辑的型号相同"
          ],
          [
            "prompt",
            "string，必填",
            "写清保留、替换或修改的内容，多图按顺序说明各图用途"
          ],
          [
            "size",
            "string，建议填写",
            "使用具体像素，遵守本页型号档位和尺寸限制"
          ],
          [
            "quality",
            "string，可选",
            "Adobe、AZ和官key按型号表选择；基础1K的整图编辑请求省略"
          ],
          [
            "n",
            "integer，建议1",
            "输出数量，不是参考图张数；Adobe支持1–4"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时传入；Adobe Image 2不发送"
          ],
          [
            "moderation",
            "string，可选",
            "基础1K、AZ和官key可选auto或low；Adobe不提供独立控制，省略"
          ],
          [
            "response_format",
            "string，可选",
            "url或b64_json；与参考图片的输入方式无关"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；实际结果按Content-Type保存"
          ],
          [
            "mask",
            "文件，可选",
            "仅支持局部编辑的型号使用；透明区域表示修改范围"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "单张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"image\": \"https://example.com/product.png\"\n}'"
      },
      {
        "type": "heading",
        "value": "多张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"images\": [\n    {\n      \"image_url\": \"https://example.com/product.png\"\n    },\n    {\n      \"image_url\": \"https://example.com/style.jpg\"\n    }\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "本地图片上传"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=gpt-image-2.5-flare' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.jpg' \\\n  -F 'size=2560x1440' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "multipart上传由HTTP客户端自动生成boundary，不手动填写Content-Type。URL必须直接返回图片文件且在排队、生成期间保持有效；不要使用image=@https://...。"
      },
      {
        "type": "heading",
        "value": "遮罩局部编辑"
      },
      {
        "type": "paragraph",
        "value": "mask与生成透明背景是不同功能。Adobe参考图与遮罩合计受64MiB请求限制；遮罩必须是带透明通道的PNG或WebP，宽高与第一张源图相同。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=gpt-image-2.5-flare' \\\n  -F 'prompt=只替换遮罩透明区域中的背景，保留人物与衣服' \\\n  -F 'image=@original.png' \\\n  -F 'mask=@mask.png' \\\n  -F 'size=1024x1024' \\\n  -F 'quality=high' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "Adobe参考图和遮罩仅使用PNG、JPEG或WebP，总请求体不超过64MiB；Base64会增大请求体积。上传本地文件时优先multipart，避免多张图片编码后超限。"
      },
      {
        "type": "heading",
        "value": "透明背景"
      },
      {
        "type": "paragraph",
        "value": "Adobe仅GPT Image 2.5 Flare和Sunburst提供背景透明度参数；GPT Image 2及gpt-image-2-4k不支持，必须省略background和transparent_background。基础1K型号也不发送background。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"gpt-image-2.5-flare\",\n  \"prompt\": \"一个独立的橘子图标，透明背景，无投影\",\n  \"size\": \"1024x1024\",\n  \"quality\": \"high\",\n  \"background\": \"transparent\",\n  \"output_format\": \"png\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图片结果字段与格式"
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "类型",
          "处理方式"
        ],
        "rows": [
          [
            "data",
            "array",
            "逐项处理生成结果，不丢弃后续图片"
          ],
          [
            "data[].url",
            "string，可选",
            "及时下载保存，临时URL不是永久存储"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "Base64解码成图片文件，不把编码文本直接写入.png"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "修订提示词，缺少此字段不等于生成失败"
          ],
          [
            "size / quality / background",
            "可选",
            "返回时记录实际参数，不凭请求值推断图片最终宽高"
          ],
          [
            "Content-Type",
            "文件下载响应头",
            "按实际MIME确定格式，不只根据URL后缀判断"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "同步响应需要在data中找到url或b64_json才算拿到图片。异步任务的result保留相同结构，有media归档地址时直接鉴权下载，客户端不需要转换接口协议。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"gpt-image-2.5-flare\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: 'gpt-image-2.5-flare', prompt: '浅色背景上的一杯橘子汽水', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "模型不存在或无可用渠道",
            "核对Key所属分组、授权范围及完整模型名；AZ使用flare，不是flara"
          ],
          [
            "response_format被拒绝",
            "AZ与官key请求省略此字段，不改成b64_json后自动重新提交"
          ],
          [
            "透明背景参数被拒绝",
            "Adobe的GPT Image 2不提供此功能；仅在本页型号表标明支持时发送"
          ],
          [
            "上传成功但参考图未生效",
            "核对image、images的线路格式及数组顺序，在prompt里说明每张参考图用途"
          ],
          [
            "连接超时或客户端关闭",
            "先查任务日志或已保存的任务编号，不自动重新创建收费任务"
          ],
          [
            "查询成功但生成失败",
            "HTTP 200只代表查询成功，仍需检查status、error和result_expired"
          ],
          [
            "价格不同",
            "同名模型在不同分组中可能使用不同计费规则和倍率，以当前Key报价和实际账单为准"
          ]
        ]
      }
    ]
  },
  {
    "id": "image-official",
    "label": "官key-image",
    "title": "官key-image · GPT Image",
    "lead": "本分组的模型名称、参数能力、比例尺寸、URL或本地参考图、图片编辑与任务处理。",
    "blocks": [
      {
        "type": "heading",
        "value": "服务器内部异步与任务恢复"
      },
      {
        "type": "paragraph",
        "value": "图片和视频请求进入服务器后台持久任务，已受理后可在账号的任务日志中查看状态、请求参数和结果。服务器内部异步与客户端选择的响应模式是两件事。"
      },
      {
        "type": "table",
        "headers": [
          "客户端方式",
          "返回和后续处理"
        ],
        "rows": [
          [
            "原协议方式：不加异步选项",
            "服务器后台执行并等待，再返回原协议响应。Images返回图片结果，Gemini返回原生内容；Videos或Midjourney仍可返回其自身的任务编号"
          ],
          [
            "显式网关任务：Prefer: respond-async或?async=true",
            "返回HTTP 202及task_id、poll_url，按poll_url查询；成功succeeded，失败failed或cancelled"
          ],
          [
            "响应头X-New-Api-Task-Id",
            "服务器内部任务编号；与视频原生id可能不同，恢复网关任务用/v1/tasks/{task_id}"
          ],
          [
            "任务日志",
            "登录创建任务所属账号，在任务日志查询；网络断开后先核对已有任务，不重复提交生成"
          ]
        ]
      },
      {
        "type": "note",
        "value": "网关任务的succeeded与视频原生任务的completed不要混用。查询使用创建账号的有效Key；MJ使用提交时的同一个Key。遵循Retry-After，保存task_id和poll_url；超时不表示服务端任务已经取消。"
      },
      {
        "type": "heading",
        "value": "接入方式与鉴权"
      },
      {
        "type": "table",
        "headers": [
          "项目",
          "填写方式"
        ],
        "rows": [
          [
            "Base URL",
            "https://please.dontblockme.men"
          ],
          [
            "文生图",
            "POST /v1/images/generations"
          ],
          [
            "图生图 / 编辑",
            "POST /v1/images/edits"
          ],
          [
            "鉴权",
            "Authorization: Bearer YOUR_API_KEY"
          ],
          [
            "分组",
            "官key-image"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "export TOKENFREE_API_KEY=\"YOUR_API_KEY\""
      },
      {
        "type": "code",
        "lang": "powershell",
        "value": "$env:TOKENFREE_API_KEY = \"YOUR_API_KEY\""
      },
      {
        "type": "paragraph",
        "value": "使用获准访问本分组的Key。同名模型可以出现在不同分组中，仅修改model不会切换Key的分组选路。普通Key使用自身分组；生图和视频Key按获授权的分组顺序匹配，需要固定某条线路时使用对应分组的普通Key。"
      },
      {
        "type": "heading",
        "value": "选择模型"
      },
      {
        "type": "table",
        "headers": [
          "模型名称",
          "尺寸档位",
          "quality",
          "background",
          "mask局部编辑",
          "线路"
        ],
        "rows": [
          [
            "官-gpt-image-2",
            "1K、2K、4K",
            "auto / low / medium / high",
            "auto / opaque / transparent",
            "支持",
            "官key-image"
          ],
          [
            "官-gpt-image-2.5-flare",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "官key-image"
          ],
          [
            "官-gpt-image-2.5-sunburst",
            "1K、2K、4K",
            "auto / low / medium / high / xhigh / max",
            "auto / opaque / transparent",
            "支持",
            "官key-image"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图参数"
      },
      {
        "type": "table",
        "headers": [
          "字段",
          "类型",
          "要求与说明"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "完整填写本页列出的模型名，保留AZ-、官-或规格后缀"
          ],
          [
            "prompt",
            "string，必填",
            "描述主体、构图、风格及画中文字，不能为空"
          ],
          [
            "size",
            "string，建议显式填写",
            "具体像素宽x高，按本页型号能力和尺寸表选择"
          ],
          [
            "quality",
            "string，可选",
            "按型号表选择；quality改变细节偏好，不改变size的格式"
          ],
          [
            "n",
            "integer，建议1",
            "建议每次1张，多张独立结果分别创建并保存任务编号"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时使用auto、opaque或transparent；Adobe的gpt-image-2及gpt-image-2-4k不发送此字段"
          ],
          [
            "response_format",
            "string，可选",
            "不发送，包括url和b64_json；此分组按官方请求协议提交，读取实际返回的data[].b64_json或任务media"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；透明背景选择png或webp"
          ],
          [
            "output_compression",
            "integer，可选",
            "JPEG/WebP压缩设置0–100；PNG省略"
          ],
          [
            "moderation",
            "string，可选",
            "auto为默认审核，low为较宽松级别，不代表关闭内容审核"
          ]
        ]
      },
      {
        "type": "note",
        "value": "AZ和官key分组不发送response_format，也不在失败后把它改成另一个值重新生成。output_format控制文件格式，不能代替response_format。"
      },
      {
        "type": "heading",
        "value": "比例、尺寸与像素限制"
      },
      {
        "type": "paragraph",
        "value": "size填写具体像素宽x高，不填写1K、4K或16:9。本页型号可使用下表的1K、2K、4K尺寸。"
      },
      {
        "type": "table",
        "headers": [
          "比例",
          "1K：size",
          "2K：size",
          "4K：size"
        ],
        "rows": [
          [
            "1:1",
            "1024x1024",
            "2048x2048",
            "2880x2880"
          ],
          [
            "3:2",
            "1536x1024",
            "2160x1440",
            "3456x2304"
          ],
          [
            "2:3",
            "1024x1536",
            "1440x2160",
            "2304x3456"
          ],
          [
            "16:9",
            "1280x720",
            "2560x1440",
            "3840x2160"
          ],
          [
            "9:16",
            "720x1280",
            "1440x2560",
            "2160x3840"
          ],
          [
            "4:3",
            "1024x768",
            "2048x1536",
            "3200x2400"
          ],
          [
            "3:4",
            "768x1024",
            "1536x2048",
            "2400x3200"
          ],
          [
            "21:9",
            "1280x544",
            "2560x1088",
            "3840x1600"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "约束",
          "要求"
        ],
        "rows": [
          [
            "宽高对齐",
            "宽和高均为16的倍数"
          ],
          [
            "最长边",
            "不超过3840px"
          ],
          [
            "长宽比",
            "最长边 / 最短边不超过3"
          ],
          [
            "总像素",
            "655360至8294400像素；同时遵守型号的档位限制"
          ],
          [
            "4K方图",
            "2880x2880，不是4096x4096"
          ],
          [
            "自定义尺寸",
            "同时满足对齐、最长边、长宽比和总像素要求；读取实际结果确认最终尺寸"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "文生图完整请求"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"官-gpt-image-2.5-flare\",\n  \"prompt\": \"清晨花店门口，柔和日光，精致商业摄影\",\n  \"size\": \"2560x1440\",\n  \"quality\": \"high\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图生图与图片编辑"
      },
      {
        "type": "paragraph",
        "value": "POST /v1/images/edits可使用本地文件或公开图片URL。单图JSON用image字符串；多图字段按下表的线路格式填写。不要把本机路径或网页预览地址当成图片URL。"
      },
      {
        "type": "table",
        "headers": [
          "输入方式",
          "Content-Type",
          "字段和限制"
        ],
        "rows": [
          [
            "本地文件",
            "multipart/form-data",
            "单图image；多图重复image[]，PNG、JPEG或WebP，最多16张参考图"
          ],
          [
            "单张URL",
            "application/json",
            "image为一张公开图片URL；与images二选一"
          ],
          [
            "多张URL",
            "application/json",
            "images为图片URL字符串数组，最多16项"
          ],
          [
            "mask",
            "multipart文件，可选",
            "支持局部编辑；透明区域为修改区域，与第一张参考图尺寸相同"
          ]
        ]
      },
      {
        "type": "table",
        "headers": [
          "编辑字段",
          "类型 / 要求",
          "填写方式"
        ],
        "rows": [
          [
            "model",
            "string，必填",
            "使用本页型号表中的完整名称，文生图和编辑的型号相同"
          ],
          [
            "prompt",
            "string，必填",
            "写清保留、替换或修改的内容，多图按顺序说明各图用途"
          ],
          [
            "size",
            "string，建议填写",
            "使用具体像素，遵守本页型号档位和尺寸限制"
          ],
          [
            "quality",
            "string，可选",
            "Adobe、AZ和官key按型号表选择；基础1K的整图编辑请求省略"
          ],
          [
            "n",
            "integer，建议1",
            "输出数量，不是参考图张数；Adobe支持1–4"
          ],
          [
            "background",
            "string，可选",
            "仅型号表标明支持时传入；Adobe Image 2不发送"
          ],
          [
            "moderation",
            "string，可选",
            "基础1K、AZ和官key可选auto或low；Adobe不提供独立控制，省略"
          ],
          [
            "response_format",
            "string，可选",
            "本分组省略，不发送url或b64_json"
          ],
          [
            "output_format",
            "string，可选",
            "png、jpeg、webp；实际结果按Content-Type保存"
          ],
          [
            "mask",
            "文件，可选",
            "仅支持局部编辑的型号使用；透明区域表示修改范围"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "单张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"官-gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"image\": \"https://example.com/product.png\"\n}'"
      },
      {
        "type": "heading",
        "value": "多张URL参考图"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"官-gpt-image-2.5-flare\",\n  \"prompt\": \"保留第一张图的商品，参考第二张图的构图，背景改成浅色\",\n  \"size\": \"2560x1440\",\n  \"n\": 1,\n  \"images\": [\n    \"https://example.com/product.png\",\n    \"https://example.com/style.jpg\"\n  ]\n}'"
      },
      {
        "type": "heading",
        "value": "本地图片上传"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=官-gpt-image-2.5-flare' \\\n  -F 'prompt=保留第一张图的商品，参考第二张图的构图' \\\n  -F 'image[]=@product.png' \\\n  -F 'image[]=@style.jpg' \\\n  -F 'size=2560x1440' \\\n  -F 'n=1'"
      },
      {
        "type": "note",
        "value": "multipart上传由HTTP客户端自动生成boundary，不手动填写Content-Type。URL必须直接返回图片文件且在排队、生成期间保持有效；不要使用image=@https://...。"
      },
      {
        "type": "heading",
        "value": "遮罩局部编辑"
      },
      {
        "type": "paragraph",
        "value": "mask与生成透明背景是不同功能。主图、遮罩各不超过50MiB；遮罩带透明通道，宽高与第一张源图相同，透明区域为修改区域。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/edits' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Prefer: respond-async' \\\n  -F 'model=官-gpt-image-2.5-flare' \\\n  -F 'prompt=只替换遮罩透明区域中的背景，保留人物与衣服' \\\n  -F 'image=@original.png' \\\n  -F 'mask=@mask.png' \\\n  -F 'size=1024x1024' \\\n  -F 'quality=high' \\\n  -F 'n=1'"
      },
      {
        "type": "heading",
        "value": "透明背景"
      },
      {
        "type": "paragraph",
        "value": "使用background:transparent并选择PNG或WebP。opaque表示不透明，auto由模型决定。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/images/generations' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  -H 'Content-Type: application/json' \\\n  -H 'Prefer: respond-async' \\\n  --data '{\n  \"model\": \"官-gpt-image-2.5-flare\",\n  \"prompt\": \"一个独立的橘子图标，透明背景，无投影\",\n  \"size\": \"1024x1024\",\n  \"quality\": \"high\",\n  \"background\": \"transparent\",\n  \"output_format\": \"png\",\n  \"n\": 1\n}'"
      },
      {
        "type": "heading",
        "value": "图片结果字段与格式"
      },
      {
        "type": "table",
        "headers": [
          "返回字段",
          "类型",
          "处理方式"
        ],
        "rows": [
          [
            "data",
            "array",
            "逐项处理生成结果，不丢弃后续图片"
          ],
          [
            "data[].url",
            "string，可选",
            "及时下载保存，临时URL不是永久存储"
          ],
          [
            "data[].b64_json",
            "string，可选",
            "Base64解码成图片文件，不把编码文本直接写入.png"
          ],
          [
            "data[].revised_prompt",
            "string，可选",
            "修订提示词，缺少此字段不等于生成失败"
          ],
          [
            "size / quality / background",
            "可选",
            "返回时记录实际参数，不凭请求值推断图片最终宽高"
          ],
          [
            "Content-Type",
            "文件下载响应头",
            "按实际MIME确定格式，不只根据URL后缀判断"
          ]
        ]
      },
      {
        "type": "paragraph",
        "value": "同步响应需要在data中找到url或b64_json才算拿到图片。异步任务的result保留相同结构，有media归档地址时直接鉴权下载，客户端不需要转换接口协议。"
      },
      {
        "type": "heading",
        "value": "异步：提交、查询与下载"
      },
      {
        "type": "paragraph",
        "value": "前面的创建示例已带Prefer: respond-async。创建成功返回HTTP 202，先保存task_id和poll_url。成功后的result仍是本页的原生响应结构，不需要在服务器额外转换协议。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"object\": \"image_generation\",\n  \"status\": \"pending\",\n  \"created\": 1790900000,\n  \"model\": \"官-gpt-image-2.5-flare\",\n  \"poll_url\": \"/v1/tasks/async_example\",\n  \"request_path\": \"/v1/images/generations\"\n}"
      },
      {
        "type": "table",
        "headers": [
          "提交字段或响应头",
          "含义"
        ],
        "rows": [
          [
            "task_id / id",
            "网关任务编号，保存后用于恢复"
          ],
          [
            "poll_url",
            "查询路径；相对路径以https://please.dontblockme.men为基准"
          ],
          [
            "Location",
            "HTTP响应头中的查询地址"
          ],
          [
            "Retry-After",
            "建议的查询等待时间；秒数或HTTP日期"
          ],
          [
            "status",
            "pending不代表生成完成"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 30 'https://please.dontblockme.men/v1/tasks/async_example' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\""
      },
      {
        "type": "code",
        "lang": "text",
        "value": "pending → processing → waiting → succeeded → 下载media\n                     ↘ failed / cancelled → 停止并检查error"
      },
      {
        "type": "paragraph",
        "value": "pending、processing、waiting继续查询同一任务；中间状态可能被跳过。succeeded读取结果，failed或cancelled停止，未知状态保留响应并排查。HTTP 200只表示查询成功。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"succeeded\",\n  \"result_expired\": false,\n  \"expires_at\": 1790986520,\n  \"response_status_code\": 200,\n  \"content_type\": \"application/json\",\n  \"result\": {\n    \"created\": 1790900000,\n    \"data\": [\n      {\n        \"b64_json\": \"<BASE64_IMAGE_DATA>\"\n      }\n    ]\n  },\n  \"media\": [\n    {\n      \"kind\": \"image\",\n      \"content_type\": \"image/png\",\n      \"url\": \"/v1/tasks/async_example/media/0\"\n    }\n  ]\n}"
      },
      {
        "type": "table",
        "headers": [
          "完成字段",
          "说明"
        ],
        "rows": [
          [
            "result",
            "生成接口的原生响应，按本页同步结构读取"
          ],
          [
            "media[].url",
            "归档文件地址，使用Bearer鉴权下载"
          ],
          [
            "media[].content_type",
            "文件格式提示，也检查下载响应的Content-Type"
          ],
          [
            "result_expired",
            "true表示归档结果过期；任务成功不等于文件仍可下载"
          ],
          [
            "expires_at",
            "结果保留截止时间，Unix秒；以实际返回为准"
          ],
          [
            "preview_url",
            "若返回则为临时展示地址，不代表永久公开文件"
          ]
        ]
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "curl --fail-with-body --max-time 120 'https://please.dontblockme.men/v1/tasks/async_example/media/0' \\\n  -H \"Authorization: Bearer $TOKENFREE_API_KEY\" \\\n  --output result.png"
      },
      {
        "type": "heading",
        "value": "失败响应与重试边界"
      },
      {
        "type": "paragraph",
        "value": "请求失败可能返回非2xx和error；已受理的任务也可能在查询时返回failed。以下只展示结构，具体错误码和文字以实际响应为准。"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"error\": {\n    \"message\": \"模型参数不匹配，请检查请求字段\",\n    \"type\": \"invalid_request_error\",\n    \"code\": \"invalid_parameter\"\n  }\n}"
      },
      {
        "type": "code",
        "lang": "json",
        "value": "{\n  \"id\": \"async_example\",\n  \"task_id\": \"async_example\",\n  \"status\": \"failed\",\n  \"error\": {\n    \"message\": \"参考素材读取失败\"\n  }\n}"
      },
      {
        "type": "table",
        "headers": [
          "状态或现象",
          "处理"
        ],
        "rows": [
          [
            "400",
            "核对模型、字段类型、尺寸和图片实际格式"
          ],
          [
            "401 / 403",
            "检查Bearer鉴权、令牌和模型权限；不要持续重试"
          ],
          [
            "413",
            "请求或参考图过大，减少素材体积"
          ],
          [
            "429 / 临时5xx",
            "查询按Retry-After退避重试；创建不盲目重发"
          ],
          [
            "404 / 410",
            "核对任务归属、路径及结果是否过期"
          ],
          [
            "failed / cancelled",
            "停止查询，记录task_id和error；处理原因后再决定是否重新创建"
          ],
          [
            "创建超时、断网",
            "先确认原任务是否已受理，不把客户端超时当成重复生成的依据"
          ]
        ]
      },
      {
        "type": "heading",
        "value": "Node.js完整示例：生成到保存文件"
      },
      {
        "type": "paragraph",
        "value": "使用Node.js 22或更新版本，无额外依赖。保存为generate.mjs，在独立目录设置TOKENFREE_API_KEY后运行。脚本只创建一次，收到任务立即保存到task.json；查询临时故障退避，结果流式下载，完整下载前使用.part扩展名。"
      },
      {
        "type": "code",
        "lang": "bash",
        "value": "node generate.mjs\n# 恢复同一任务，不重复生成\nnode generate.mjs task.json"
      },
      {
        "type": "note",
        "value": "30分钟为客户端等待期限，不是服务端生成时限。超时后可继续查询原任务。已有task.json时直接运行创建命令会停止，避免覆盖编号；确需新任务请使用新目录。"
      },
      {
        "type": "code",
        "lang": "javascript",
        "value": "import { readFile, writeFile, rename, rm } from 'node:fs/promises';\nimport { createWriteStream, existsSync } from 'node:fs';\nimport { Readable } from 'node:stream';\nimport { pipeline } from 'node:stream/promises';\nimport { setTimeout as delay } from 'node:timers/promises';\n\nconst ORIGIN = 'https://please.dontblockme.men';\nconst key = process.env.TOKENFREE_API_KEY;\nif (!key) throw new Error('请设置 TOKENFREE_API_KEY');\nconst headers = { Authorization: 'Bearer ' + key };\n\n// 只向本站地址发送 Key，不把鉴权头转发给第三方结果地址。\nfunction ownUrl(path) {\n  const url = new URL(path, ORIGIN);\n  if (url.origin !== ORIGIN) throw new Error('收到非本站的鉴权接口地址');\n  return url;\n}\nasync function readJson(response) {\n  const text = await response.text();\n  let value;\n  try { value = JSON.parse(text); }\n  catch { throw new Error('接口未返回 JSON，HTTP ' + response.status); }\n  if (!response.ok) throw new Error(value.error?.message || value.message || 'HTTP ' + response.status);\n  return value;\n}\nfunction nextInterval(response) {\n  const value = response.headers.get('Retry-After');\n  if (!value) return 3000;\n  const seconds = Number(value);\n  const ms = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now();\n  return Number.isFinite(ms) && ms > 0 ? Math.max(ms, 1000) : 3000;\n}\n\nlet submitted;\nlet interval = 3000;\nif (process.argv[2]) {\n  // 传入已保存的任务文件时只恢复查询，不重新生成或扣费。\n  submitted = JSON.parse(await readFile(process.argv[2], 'utf8'));\n} else {\n  if (existsSync('task.json')) throw new Error('已有task.json，请传入此文件恢复，或在新目录创建新任务');\n  const response = await fetch(ORIGIN + '/v1/images/generations', {\n    method: 'POST',\n    headers: { ...headers, 'Content-Type': 'application/json', Prefer: 'respond-async' },\n    body: JSON.stringify({ model: '官-gpt-image-2.5-flare', prompt: '浅色背景上的一杯橘子汽水', n: 1 }),\n    redirect: 'error',\n    signal: AbortSignal.timeout(120000)\n  });\n  submitted = await readJson(response);\n  if (response.status !== 202) throw new Error('未收到预期的异步任务响应');\n  interval = nextInterval(response);\n  console.log('请保留任务编号：', submitted.task_id);\n  await writeFile('task.json', JSON.stringify(submitted, null, 2), { flag: 'wx' });\n  console.log('任务已保存：', submitted.task_id);\n}\nif (!submitted.poll_url || !submitted.task_id) throw new Error('任务缺少 task_id 或 poll_url');\nconst pollUrl = ownUrl(submitted.poll_url);\nconst deadline = Date.now() + 30 * 60 * 1000;\nlet task;\nwhile (Date.now() < deadline) {\n  // 尊重服务端的查询间隔；超过本地等待期限后仍保留任务编号。\n  if (Date.now() + interval >= deadline) break;\n  await delay(interval);\n  let check;\n  try {\n    check = await fetch(pollUrl, {\n      headers, redirect: 'error', signal: AbortSignal.timeout(30000)\n    });\n  } catch {\n    // 查询网络错误只重试查询，绝不重新提交生成。\n    interval = Math.min(interval * 2, 30000);\n    continue;\n  }\n  if (check.status === 429 || check.status >= 500) {\n    await check.body?.cancel();\n    interval = Math.max(nextInterval(check), Math.min(interval * 2, 30000));\n    continue;\n  }\n  interval = nextInterval(check);\n  task = await readJson(check);\n  if (task.status === 'succeeded') break;\n  if (['failed', 'cancelled'].includes(task.status)) {\n    throw new Error(task.error?.message || task.status);\n  }\n  if (!['pending', 'processing', 'waiting'].includes(task.status)) {\n    throw new Error('未识别的任务状态：' + task.status);\n  }\n}\nif (task?.status !== 'succeeded') throw new Error('等待已结束，可运行 node generate.mjs task.json 恢复查询');\nif (task.result_expired) throw new Error('结果文件已过期');\nawait writeFile('result.json', JSON.stringify(task.result ?? task, null, 2));\nconst files = task.media ?? [];\nfor (const [index, item] of files.entries()) {\n  const file = await fetch(ownUrl(item.url), {\n    headers, redirect: 'error', signal: AbortSignal.timeout(120000)\n  });\n  if (!file.ok) throw new Error('下载失败，HTTP ' + file.status);\n  if (!file.body) throw new Error('下载响应缺少文件内容');\n  const mime = item.content_type || file.headers.get('Content-Type') || '';\n  const ext = ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'video/mp4': 'mp4' })[mime.split(';')[0].trim()] || 'bin';\n  // 流式写入图片和视频，避免把整个视频文件一次性加载到内存。\n  const name = 'result-' + index + '.' + ext;\n  try {\n    await pipeline(Readable.fromWeb(file.body), createWriteStream(name + '.part'));\n    await rename(name + '.part', name);\n  } catch (error) {\n    await rm(name + '.part', { force: true });\n    throw error;\n  }\n}\nif (!files.length) throw new Error('没有归档媒体，请检查result.json中的原生图片结果或失败原因');\nconsole.log('文件已保存');\n"
      },
      {
        "type": "heading",
        "value": "常见问题"
      },
      {
        "type": "table",
        "headers": [
          "情况",
          "处理方式"
        ],
        "rows": [
          [
            "模型不存在或无可用渠道",
            "核对Key所属分组、授权范围及完整模型名；AZ使用flare，不是flara"
          ],
          [
            "response_format被拒绝",
            "AZ与官key请求省略此字段，不改成b64_json后自动重新提交"
          ],
          [
            "透明背景参数被拒绝",
            "Adobe的GPT Image 2不提供此功能；仅在本页型号表标明支持时发送"
          ],
          [
            "上传成功但参考图未生效",
            "核对image、images的线路格式及数组顺序，在prompt里说明每张参考图用途"
          ],
          [
            "连接超时或客户端关闭",
            "先查任务日志或已保存的任务编号，不自动重新创建收费任务"
          ],
          [
            "查询成功但生成失败",
            "HTTP 200只代表查询成功，仍需检查status、error和result_expired"
          ],
          [
            "价格不同",
            "同名模型在不同分组中可能使用不同计费规则和倍率，以当前Key报价和实际账单为准"
          ]
        ]
      }
    ]
  }
];
export const navigation = [
  {
    "id": "model-groups",
    "label": "模型分组",
    "pages": [
      "image-1k",
      "banana",
      "image-az",
      "image-adobe",
      "image-official",
      "image2"
    ]
  },
  {
    "page": "seedream"
  },
  {
    "page": "mj-v8.2"
  },
  {
    "page": "video"
  },
  {
    "page": "sd-video"
  }
];
