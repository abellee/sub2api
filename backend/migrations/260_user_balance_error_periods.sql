-- First local balance rejection per user/credit period, independent of log retention.
CREATE TABLE user_balance_error_periods (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credited_at TIMESTAMPTZ NOT NULL,
    first_error_id BIGINT,
    PRIMARY KEY (user_id, credited_at)
);

CREATE FUNCTION start_user_balance_error_period() RETURNS trigger AS $$
BEGIN
    INSERT INTO user_balance_error_periods(user_id, credited_at)
    VALUES (NEW.id, clock_timestamp());
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_balance_error_credit
AFTER UPDATE OF balance ON users FOR EACH ROW
WHEN (NEW.balance > OLD.balance)
EXECUTE FUNCTION start_user_balance_error_period();

CREATE FUNCTION keep_first_user_balance_error() RETURNS trigger AS $$
DECLARE
    period_start TIMESTAMPTZ;
    first_id BIGINT;
BEGIN
    IF NEW.error_type <> 'insufficient_balance' OR NEW.user_id IS NULL
       OR NEW.error_owner <> 'client' OR NEW.error_phase <> 'request' THEN
        RETURN NEW;
    END IF;
    -- Delayed pre-credit logs must not consume the next period's first rejection.
    SELECT credited_at INTO period_start FROM user_balance_error_periods
    WHERE user_id = NEW.user_id AND credited_at <= NEW.created_at
    ORDER BY credited_at DESC LIMIT 1;
    period_start := COALESCE(period_start, '-infinity'::timestamptz);
    INSERT INTO user_balance_error_periods(user_id, credited_at, first_error_id)
    VALUES (NEW.user_id, period_start, NEW.id)
    ON CONFLICT (user_id, credited_at) DO UPDATE
    SET first_error_id = COALESCE(user_balance_error_periods.first_error_id, EXCLUDED.first_error_id)
    RETURNING first_error_id INTO first_id;
    IF first_id <> NEW.id THEN
        RETURN NULL;
    END IF;
    NEW.is_business_limited := true;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ops_user_balance_error_period
BEFORE INSERT ON ops_error_logs FOR EACH ROW
EXECUTE FUNCTION keep_first_user_balance_error();
