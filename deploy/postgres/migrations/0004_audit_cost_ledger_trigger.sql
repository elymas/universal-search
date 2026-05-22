-- SPEC-AUTH-003 Phase C: cost_ledger AFTER INSERT trigger for audit mirroring.
-- REQ-AUTH3-002: cost_ledger mirror via Postgres AFTER INSERT trigger.
-- D5: DEEP-004 Go code unchanged.
-- @MX:WARN: [AUTO] Trigger failure can abort cost_ledger INSERT when audit.cost_mirror_strict = true
-- @MX:REASON: DEEP-004 cost write path depends on this trigger. Modifying the trigger requires DEEP-004 operational impact assessment.

-- Create the trigger function that mirrors cost_ledger rows to audit_events.
CREATE OR REPLACE FUNCTION cost_ledger_to_audit()
RETURNS TRIGGER AS $$
DECLARE
    strict_mode BOOLEAN;
BEGIN
    -- Read strict mode from app_config or default to true.
    -- If the config table doesn't exist, default to true.
    BEGIN
        SELECT COALESCE(
            (SELECT value::boolean FROM app_config WHERE key = 'audit.cost_mirror_strict'),
            true
        ) INTO strict_mode;
    EXCEPTION WHEN undefined_table THEN
        strict_mode := true;
    END;

    BEGIN
        INSERT INTO audit_events (
            event_type, decision, user_id, tenant_id, request_id,
            source, resource, action, payload, ts
        ) VALUES (
            'cost.recorded',
            'none',
            COALESCE(NEW.user_id, 'anonymous'),
            COALESCE(NEW.tenant_id, 'default'),
            NEW.request_id,
            'go',
            'cost_ledger',
            'record',
            jsonb_build_object(
                'cost_ledger_id', NEW.id,
                'model', NEW.model,
                'prompt_tokens', NEW.prompt_tokens,
                'completion_tokens', NEW.completion_tokens,
                'usd_cost', NEW.usd_cost,
                'cache_hit', NEW.cache_hit,
                'outcome', NEW.outcome
            ),
            COALESCE(NEW.ts, NOW())
        );
    EXCEPTION WHEN OTHERS THEN
        -- If strict mode, re-raise to abort the cost_ledger INSERT.
        IF strict_mode THEN
            RAISE EXCEPTION 'audit: failed to mirror cost_ledger row %: %', NEW.id, SQLERRM;
        END IF;
        -- Otherwise, just log the error and continue.
        RAISE WARNING 'audit: failed to mirror cost_ledger row %: %', NEW.id, SQLERRM;
    END;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop existing trigger if re-running (idempotent).
DROP TRIGGER IF EXISTS cost_ledger_to_audit_trigger ON cost_ledger;

-- Create the AFTER INSERT trigger.
CREATE TRIGGER cost_ledger_to_audit_trigger
    AFTER INSERT ON cost_ledger
    FOR EACH ROW EXECUTE FUNCTION cost_ledger_to_audit();
