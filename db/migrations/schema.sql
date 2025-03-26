CREATE TABLE "users" (
    "id" uuid DEFAULT uuid_generate_v4(),
    "username" varchar NOT NULL UNIQUE,
    "password" varchar NOT NULL,
    "email" varchar NOT NULL DEFAULT '' UNIQUE,
    PRIMARY KEY ("id")
);

CREATE TABLE "resumes" (
    "id" uuid DEFAULT uuid_generate_v4(),
    "user_id" uuid NOT NULL,
    "job_posting_id" uuid NOT NULL,
    "resume_pdf" BYTEA NOT NULL,
    "parsed_resume" JSONB ,
    PRIMARY KEY ("id")
);

CREATE TABLE job_postings (
    "id" uuid DEFAULT uuid_generate_v4(),
    "recruiter_id" uuid NOT NULL,
    "title" VARCHAR NOT NULL,
    "salary_min" numeric(10,2),
    "salary_max" numeric(10,2),
    "status" varchar(20) NOT NULL DEFAULT 'active',
    PRIMARY KEY ("id")
);


ALTER TABLE "resumes" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");
ALTER TABLE "resumes" ADD FOREIGN KEY ("job_posting_id") REFERENCES "job_postings" ("id");
ALTER TABLE "job_postings" ADD FOREIGN KEY ("recruiter_id") REFERENCES "users" ("id");
