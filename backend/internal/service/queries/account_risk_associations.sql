WITH active_users AS MATERIALIZED (
    SELECT id,email,registration_ip,registration_user_agent,created_at,last_login_ip,last_login_user_agent,last_login_at
    FROM users WHERE deleted_at IS NULL
), signals(user_id,kind,value,reason,first_seen,last_seen) AS (
    SELECT id,'ip',registration_ip,'registration_ip',created_at,created_at FROM active_users
      WHERE registration_ip<>'' AND created_at >= $1
    UNION ALL SELECT id,'ip',last_login_ip,'login_ip',last_login_at,last_login_at FROM active_users
      WHERE last_login_ip<>'' AND last_login_at >= $1
    UNION ALL SELECT user_id,'ip',client_ip,'payment_ip',MIN(created_at),MAX(created_at) FROM payment_orders
      WHERE client_ip<>'' AND created_at >= $1 GROUP BY user_id,client_ip
    UNION ALL SELECT user_id,'ip',ip_address,'usage_ip',MIN(first_seen_at),MAX(last_seen_at) FROM account_risk_usage_ips
      WHERE bucket >= date_trunc('hour',$2::timestamptz) AND last_seen_at >= $2 GROUP BY user_id,ip_address
    UNION ALL SELECT e.user_id,s.kind,s.value,s.reason,MIN(e.created_at),MAX(e.created_at)
      FROM account_risk_events e CROSS JOIN LATERAL (VALUES
        ('browser',e.browser_hash,'browser'),
        ('ip',e.ip_address,CASE e.event_type WHEN 'register' THEN 'registration_ip' WHEN 'login' THEN 'login_ip' ELSE 'checkin_ip' END)
      ) s(kind,value,reason)
      WHERE s.value<>'' AND e.created_at >= $1 GROUP BY e.user_id,s.kind,s.value,s.reason
), points AS MATERIALIZED (
    SELECT s.user_id,kind,value,reason,MIN(first_seen) first_seen,MAX(last_seen) last_seen,
           (strpos(lower(u.email),lower($3))>0 OR $3='') AS searched
    FROM signals s JOIN active_users u ON u.id=s.user_id GROUP BY s.user_id,u.email,kind,value,reason
), matches AS (
    SELECT LEAST(a.user_id,b.user_id) AS a,GREATEST(a.user_id,b.user_id) AS b,matched.reason,GREATEST(a.first_seen,b.first_seen) AS first_seen,
           GREATEST(a.last_seen,b.last_seen) AS last_seen
    FROM points a JOIN points b ON a.kind=b.kind AND a.value=b.value AND a.user_id<>b.user_id
    CROSS JOIN LATERAL (VALUES(a.reason),(b.reason)) matched(reason)
    WHERE a.searched AND (NOT b.searched OR a.user_id<b.user_id)
    UNION ALL
    SELECT LEAST(inviter.id,invitee.id),GREATEST(inviter.id,invitee.id),'invitation',aff.created_at,aff.created_at
    FROM user_affiliates aff JOIN active_users inviter ON inviter.id=aff.inviter_id JOIN active_users invitee ON invitee.id=aff.user_id
    WHERE inviter.id<>invitee.id AND aff.created_at >= $1
      AND ($3='' OR strpos(lower(inviter.email),lower($3))>0 OR strpos(lower(invitee.email),lower($3))>0)
), reasons AS (
    SELECT a,b,reason,MIN(first_seen) first_seen,MAX(last_seen) last_seen FROM matches GROUP BY a,b,reason
), pairs AS (
    SELECT a,b,array_agg(reason ORDER BY reason) reasons,MIN(first_seen) first_seen,MAX(last_seen) last_seen,
           jsonb_agg(jsonb_build_object('reason',reason,'first_seen_at',first_seen,'last_seen_at',last_seen) ORDER BY reason) evidence
    FROM reasons GROUP BY a,b
), reviews AS (
    SELECT DISTINCT ON (LEAST(c.user_id,r.user_id),GREATEST(c.user_id,r.user_id))
        LEAST(c.user_id,r.user_id) a,GREATEST(c.user_id,r.user_id) b,r.status
    FROM checkin_browser_reviews r JOIN checkin_browser_claims c USING (checkin_date,browser_hash)
    WHERE r.created_at >= $1 ORDER BY 1,2,r.created_at DESC,r.checkin_date DESC,r.browser_hash
), filtered AS MATERIALIZED (
    SELECT p.*,COALESCE(reviews.status,'none') status FROM pairs p LEFT JOIN reviews USING (a,b)
    WHERE ($4='' OR $4=ANY(reasons)) AND ($5='' OR $5=COALESCE(reviews.status,'none'))
      AND ($6::timestamptz IS NULL OR last_seen >= $6) AND ($7::timestamptz IS NULL OR last_seen < $7)
), page AS (
    SELECT * FROM filtered ORDER BY last_seen DESC,a,b LIMIT $8 OFFSET $9
), details AS (
    SELECT u.email user_email,v.email related_email,p.reasons,p.first_seen first_seen_at,p.last_seen last_seen_at,p.evidence,p.status,
        COALESCE(inviter.email,'') inviter_email,
        ('browser'=ANY(p.reasons) AND (inviter.email IS NOT NULL OR p.reasons && ARRAY['registration_ip','usage_ip','login_ip','payment_ip','checkin_ip'])) high_risk,
        mine.count checkin_count,mine.reward reward_amount,theirs.count related_checkin_count,theirs.reward related_reward_amount,
        COALESCE(latest_ua.user_agent,NULLIF(u.last_login_user_agent,''),u.registration_user_agent,'') user_agent,
        COALESCE(related_ua.user_agent,NULLIF(v.last_login_user_agent,''),v.registration_user_agent,'') related_user_agent
    FROM page p JOIN active_users u ON u.id=p.a JOIN active_users v ON v.id=p.b
    LEFT JOIN LATERAL (
       SELECT inviter.email FROM user_affiliates aff JOIN active_users inviter ON inviter.id=aff.inviter_id
       WHERE (aff.user_id=p.a AND aff.inviter_id=p.b) OR (aff.user_id=p.b AND aff.inviter_id=p.a) ORDER BY aff.user_id LIMIT 1
    ) inviter ON true
    CROSS JOIN LATERAL (SELECT COUNT(*) count, COALESCE(SUM(reward_amount),0) reward FROM user_checkins WHERE user_id=p.a AND created_at >= $1) mine
    CROSS JOIN LATERAL (SELECT COUNT(*) count, COALESCE(SUM(reward_amount),0) reward FROM user_checkins WHERE user_id=p.b AND created_at >= $1) theirs
    LEFT JOIN LATERAL (SELECT e.user_agent FROM account_risk_events e WHERE e.user_id=p.a AND e.created_at >= $1 AND user_agent<>'' ORDER BY created_at DESC,id DESC LIMIT 1) latest_ua ON true
    LEFT JOIN LATERAL (SELECT e.user_agent FROM account_risk_events e WHERE e.user_id=p.b AND e.created_at >= $1 AND user_agent<>'' ORDER BY created_at DESC,id DESC LIMIT 1) related_ua ON true
    ORDER BY p.last_seen DESC,p.a,p.b
)
SELECT (SELECT COUNT(*) FROM filtered),COALESCE(jsonb_agg(to_jsonb(details)),'[]'::jsonb) FROM details
