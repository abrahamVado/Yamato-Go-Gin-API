CREATE TABLE mark_type (
  id          BIGSERIAL      PRIMARY KEY,
  title       VARCHAR(150)   NOT NULL,
  slug        VARCHAR(150)   NOT NULL UNIQUE,
  url         TEXT,
  description TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by  BIGINT
);

-- Logical “file group” for a report (can be 0..N physical files)
CREATE TABLE tmfile (
  id          BIGSERIAL PRIMARY KEY,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Concrete files (images, videos…) attached to tmfile
CREATE TABLE tdfile (
  id          BIGSERIAL PRIMARY KEY,
  tmfile_id   BIGINT      NOT NULL REFERENCES tmfile(id) ON DELETE CASCADE,
  file_type   VARCHAR(16) NOT NULL CHECK (file_type IN ('image', 'video')),
  size_bytes  BIGINT      NOT NULL,              -- raw size in bytes
  format      VARCHAR(64) NOT NULL,              -- e.g. 'image/png', 'video/mp4'
  url         TEXT        NOT NULL,              -- where the file is stored
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tdfile_tmfile_idx ON tdfile (tmfile_id);


-- Reports created by users / devices
CREATE TABLE report (
  id               BIGSERIAL PRIMARY KEY,
  title            VARCHAR(200) NOT NULL,
  description      TEXT,
  supported        BOOLEAN      NOT NULL DEFAULT FALSE,  -- e.g. “I support this report”
  visited          BOOLEAN      NOT NULL DEFAULT FALSE,  -- e.g. “I already went there”
  tmfile_id        BIGINT REFERENCES tmfile(id) ON DELETE SET NULL,
  created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  device_token_id  BIGINT,                               -- FK to your device_tokens table if exists
  gps              POINT                                   -- (lng, lat) or (x, y)
);

CREATE INDEX report_tmfile_idx ON report (tmfile_id);
CREATE INDEX report_device_token_idx ON report (device_token_id);
CREATE INDEX report_gps_idx ON report USING GIST (gps);