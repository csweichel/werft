
CREATE TABLE IF NOT EXISTS job_status (
	id SERIAL PRIMARY KEY,
	name varchar(255) NOT NULL UNIQUE,
	data text NOT NULL,
	owner varchar(255) NULL,
	phase VARCHAR(255) NOT NULL,
	repo_owner varchar(255) NULL,
	repo_repo varchar(255) NULL,
	repo_host varchar(255) NULL,
	repo_ref varchar(255) NULL,
	trigger_src varchar(255) NULL,
	success int not null,
	created int not null
);

CREATE TABLE IF NOT EXISTS annotations (
	job_id INT NOT NULL,
	name varchar(255) NOT NULL,
	value text NULL,
	CONSTRAINT job_annotation UNIQUE(job_id, name)
);

CREATE TABLE IF NOT EXISTS number_group (
	name varchar(255) NOT NULL PRIMARY KEY,
	val int NOT NULL
);
