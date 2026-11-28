# Application SSE errors are resolved by the backend; never buffer normal SSE here.
header_filter_by_lua_block {
    local handled = ngx.header["X-Sub2api-Error-Handled"] == "1"
    ngx.header["X-Sub2api-Error-Handled"] = nil
    local status = ngx.status
    if status < 400 or status >= 600 then return end

    local origin = ngx.var.http_origin
    for name, value in pairs(ngx.resp.get_headers(0, true)) do
        local key = string.lower(name)
        local keep = false
        if type(value) == "string" then
            if key == "access-control-allow-origin" then
                keep = value == "*" or value == "null" or (origin and value == origin)
            elseif key == "access-control-allow-credentials" then
                keep = value == "true"
            elseif key == "access-control-allow-methods" or key == "access-control-allow-headers"
                or key == "access-control-expose-headers" then
                keep = not string.find(value, "[^%w_%-, *]")
            elseif key == "access-control-max-age" or key == "retry-after" then
                keep = string.match(value, "^%d+$") ~= nil
            elseif key == "x-request-id" then
                keep = string.match(value, "^[%x]+%-%x+%-%x+%-%x+%-%x+$") ~= nil
            elseif handled and (key == "content-type" or key == "content-encoding") then
                keep = true
            end
        end
        if not keep then ngx.header[name] = nil end
    end
    ngx.header.content_length = nil
    if handled or ngx.ctx.cn_text_only_error then return end
    ngx.header.content_encoding = nil
    ngx.ctx.sub2api_fixed_error = true
    ngx.header.content_type = "application/json; charset=utf-8"
    ngx.header.cache_control = "no-store"
}
body_filter_by_lua_block {
    local ctx = ngx.ctx
    if ctx.cn_text_only_error then
        ngx.arg[1] = ctx.cn_text_only_error_sent and "" or require("cjson.safe").encode({
            error = {type = "media_generation_disabled", message = ctx.cn_text_only_error}
        })
        ngx.arg[2] = true
        ctx.cn_text_only_error_sent = true
    elseif ctx.sub2api_fixed_error then
        ngx.arg[1] = ctx.sub2api_error_sent and "" or [=[{"error":{"type":"upstream_error","message":"当前服务拥挤，请重试。"}}]=]
        ngx.arg[2] = true
        ctx.sub2api_error_sent = true
    end
}
