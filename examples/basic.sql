CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users VALUES (1, 'Adarsh', true);
SELECT * FROM users;
UPDATE users SET active = false WHERE id = 1;
