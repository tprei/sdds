DROP INDEX user_login_identities_one_password_provider_per_user_idx;

CREATE UNIQUE INDEX user_login_identities_one_local_identity_per_user_idx
	ON user_login_identities (user_id)
	WHERE provider = 'local';

CREATE UNIQUE INDEX user_login_identities_local_username_idx
	ON user_login_identities (normalized_identifier)
	WHERE provider = 'local';
