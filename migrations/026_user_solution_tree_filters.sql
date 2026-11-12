CREATE TABLE IF NOT EXISTS user_solution_tree_filters (
    user_id       UUID NOT NULL,
    solution_id   UUID NOT NULL,
    mode          TEXT NOT NULL,
    node_xml_ids  TEXT[] NOT NULL DEFAULT '{}',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, solution_id),
    CONSTRAINT user_solution_tree_filters_user_solution_fkey
        FOREIGN KEY (user_id, solution_id)
        REFERENCES user_solutions (user_id, solution_id)
        ON DELETE CASCADE,
    CONSTRAINT user_solution_tree_filters_mode_chk
        CHECK (mode IN ('all', 'schemes', 'projects'))
);
