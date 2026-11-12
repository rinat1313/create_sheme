UPDATE user_solution_tree_filters
SET mode = 'schemes'
WHERE mode = 'projects';

ALTER TABLE user_solution_tree_filters
    DROP CONSTRAINT IF EXISTS user_solution_tree_filters_mode_chk;

ALTER TABLE user_solution_tree_filters
    ADD CONSTRAINT user_solution_tree_filters_mode_chk
        CHECK (mode IN ('all', 'schemes'));
