CREATE TABLE "users" (
    "id" uuid DEFAULT gen_random_uuid(),
    "google_id" varchar NOT NULL DEFAULT '' UNIQUE,
    "email" varchar NOT NULL DEFAULT '' UNIQUE,
    "name" varchar NOT NULL UNIQUE,
    "role" varchar NOT NULL DEFAULT 'applicant',
    PRIMARY KEY ("id")
);

CREATE TABLE "resumes" (
    "id" uuid DEFAULT gen_random_uuid(),
    "user_id" uuid NOT NULL,
    "job_posting_id" uuid NOT NULL,
    "resume_pdf" BYTEA NOT NULL,
    "parsed_resume" JSONB ,
    PRIMARY KEY ("id")
);

CREATE TABLE job_postings (
    "id" uuid DEFAULT gen_random_uuid(),
    "recruiter_id" uuid NOT NULL,
    "title" VARCHAR NOT NULL,
    "salary_min" numeric(10,2),
    "salary_max" numeric(10,2),
    "status" varchar(20) NOT NULL DEFAULT 'active',
    PRIMARY KEY ("id")
);

CREATE TABLE "skills" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "name" varchar NOT NULL UNIQUE 
);

 CREATE TABLE "user_skills" (
    "user_id" uuid NOT NULL REFERENCES "users"("id") ON DELETE CASCADE,
    "skill_id" uuid NOT NULL REFERENCES "skills"("id") ON DELETE CASCADE,
    PRIMARY KEY ("user_id", "skill_id") 
);

CREATE TABLE "applications" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "user_id" uuid NOT NULL REFERENCES "users"("id") ON DELETE CASCADE, 
    "job_posting_id" uuid NOT NULL REFERENCES "job_postings"("id") ON DELETE CASCADE,
    "resume_id" uuid REFERENCES "resumes"("id") ON DELETE SET NULL, 
    "status" varchar NOT NULL DEFAULT 'submitted', 
    "applied_at" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("user_id", "job_posting_id") 
);

CREATE TABLE "interviews" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "application_id" uuid NOT NULL UNIQUE REFERENCES "applications"("id") ON DELETE CASCADE, 
    "requesting_user_id" uuid NOT NULL REFERENCES "users"("id"), 
    "proposed_details" text, 
    "status" varchar NOT NULL DEFAULT 'requested' 
);
ALTER TABLE "resumes" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");
ALTER TABLE "resumes" ADD FOREIGN KEY ("job_posting_id") REFERENCES "job_postings" ("id");
ALTER TABLE "job_postings" ADD FOREIGN KEY ("recruiter_id") REFERENCES "users" ("id");
