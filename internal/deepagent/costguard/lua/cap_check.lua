-- cap_check.lua: Atomic cap-check + counter increment + TTL refresh.
-- SPEC-DEEP-004 REQ-DEEP4-009: single Redis call covers eval + increment + TTL refresh.
--
-- KEYS[1] = costguard:calls:tenant:{tenant_id}
-- KEYS[2] = costguard:window:tenant:{tenant_id}
-- KEYS[3] = costguard:calls:user:{user_id}       (optional, empty string if user cap disabled)
-- KEYS[4] = costguard:window:user:{user_id}       (optional, empty string if user cap disabled)
--
-- ARGV[1] = tenant_max_calls
-- ARGV[2] = tenant_max_usd
-- ARGV[3] = user_max_calls   (0 if user cap disabled)
-- ARGV[4] = user_max_usd     (0 if user cap disabled)
-- ARGV[5] = cost_this_call_usd
-- ARGV[6] = ttl_seconds

local function evaluate_cap(calls_key, usd_key, max_calls, max_usd, cost_usd, ttl)
    local calls = tonumber(redis.call('GET', calls_key) or '0')
    local usd = tonumber(redis.call('GET', usd_key) or '0')

    -- Check if cap already exceeded BEFORE incrementing.
    if max_calls > 0 and calls >= max_calls then
        return {
            0,   -- allowed = false
            1,   -- dimension = calls
            calls,
            usd,
            max_calls - calls,
            max_usd - usd
        }
    end

    if max_usd > 0 and (usd + cost_usd) > max_usd then
        return {
            0,   -- allowed = false
            2,   -- dimension = usd
            calls,
            usd,
            max_calls - calls,
            max_usd - usd
        }
    end

    -- Cap not exceeded: increment counters and refresh TTL.
    local new_calls = redis.call('INCR', calls_key)
    local new_usd = redis.call('INCRBYFLOAT', usd_key, cost_usd)
    redis.call('EXPIRE', calls_key, ttl)
    redis.call('EXPIRE', usd_key, ttl)

    return {
        1,            -- allowed = true
        0,            -- dimension = none
        new_calls,
        new_usd,
        max_calls - new_calls,
        max_usd - new_usd
    }
end

-- Evaluate tenant cap first.
local tenant_result = evaluate_cap(
    KEYS[1], KEYS[2],
    tonumber(ARGV[1]), tonumber(ARGV[2]),
    tonumber(ARGV[5]), tonumber(ARGV[6])
)

if tenant_result[1] == 0 then
    return tenant_result
end

-- If user cap is enabled (ARGV[3] > 0), evaluate user cap.
if tonumber(ARGV[3]) > 0 and KEYS[3] ~= '' then
    local user_result = evaluate_cap(
        KEYS[3], KEYS[4],
        tonumber(ARGV[3]), tonumber(ARGV[4]),
        tonumber(ARGV[5]), tonumber(ARGV[6])
    )
    if user_result[1] == 0 then
        return user_result
    end
end

return tenant_result
