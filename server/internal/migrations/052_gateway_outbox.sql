-- Outbound chat relay for the messaging gateway. The app enqueues replies
-- here; the frwrd daemon polls and delivers them into the conversation
-- thread, which closes the loop for two-way chat from the board.
CREATE TABLE gateway_outbox (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	thread text NOT NULL,
	body text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gateway_outbox_user_poll_idx ON gateway_outbox (user_id, created_at);
