package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ghduuep/pingly/internal/dto"
	"github.com/ghduuep/pingly/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateUser(ctx context.Context, db *pgxpool.Pool, user *models.User) error {
	query := `INSERT INTO users(username, email, password_hash, created_at) VALUES($1, $2, $3, $4) RETURNING id`

	err := db.QueryRow(ctx, query, user.Username, user.Email, user.PasswordHash, user.CreatedAt).Scan(&user.ID)
	return err
}

func DeleteUser(ctx context.Context, db *pgxpool.Pool, id int) error {
	query := `DELETE FROM users WHERE id = $1`

	_, err := db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

func GetUserEmailByID(ctx context.Context, db *pgxpool.Pool, id int) (string, error) {
	query := `SELECT email FROM users WHERE id=$1`

	var email string
	err := db.QueryRow(ctx, query, id).Scan(&email)
	if err != nil {
		return "", err
	}
	return email, nil
}

func GetUserByID(ctx context.Context, db *pgxpool.Pool, id int) (models.User, error) {
	query := `SELECT * FROM users WHERE id=$1`
	var user models.User
	err := db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func GetUserByUsername(ctx context.Context, db *pgxpool.Pool, username string) (models.User, error) {
	query := `SELECT * FROM users WHERE username = $1`

	var user models.User
	err := db.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return models.User{}, err
	}

	return user, err
}

func UpdateUser(ctx context.Context, db *pgxpool.Pool, userID int, data dto.UpdateUserRequest) error {

	query, args, err := buildUpdateQuery(userID, data)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

func GetUserChannels(ctx context.Context, db *pgxpool.Pool, userID int) ([]models.NotificationChannel, error) {
	query := `SELECT id, user_id, type, target, enabled FROM user_channels WHERE user_id = $1`

	rows, err := db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.NotificationChannel])
	if err != nil {
		return nil, err
	}

	return channels, nil
}

func CreateChannel(ctx context.Context, db *pgxpool.Pool, channel *models.NotificationChannel) error {
	query := `INSERT INTO user_channels (user_id, type, target) VALUES ($1, $2, $3) RETURNING id`

	err := db.QueryRow(ctx, query, channel.UserID, channel.Type, channel.Target).Scan(&channel.ID)
	return err
}

func DeleteChannel(ctx context.Context, db *pgxpool.Pool, channelID int, userID int) error {
	query := `DELETE FROM user_channels WHERE id = $1 AND user_id = $2`

	_, err := db.Exec(ctx, query, channelID, userID)
	if err != nil {
		return err
	}

	return nil
}

func GetEnabledUserChannels(ctx context.Context, db *pgxpool.Pool, userID int) ([]models.NotificationChannel, error) {
	query := `SELECT id, user_id, type, target, enabled FROM user_channels WHERE user_id = $1 AND enabled = true`

	rows, err := db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.NotificationChannel])
	if err != nil {
		return nil, err
	}

	return channels, nil
}

func UpdateChannel(ctx context.Context, db *pgxpool.Pool, channelID, userID int, req dto.UpdateChannelRequest) error {
	var setParts []string
	var args []any
	argID := 1

	if req.Type != nil {
		setParts = append(setParts, fmt.Sprintf("type = $%d", argID))
		args = append(args, *req.Type)
		argID++
	}

	if req.Target != nil {
		setParts = append(setParts, fmt.Sprintf("target = $%d", argID))
		args = append(args, *req.Target)
		argID++
	}

	if req.Enabled != nil {
		setParts = append(setParts, fmt.Sprintf("enabled = $%d", argID))
		args = append(args, *req.Enabled)
		argID++
	}

	if len(setParts) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE user_channels SET %s WHERE id = $%d AND user_id = $%d", strings.Join(setParts, ", "), argID, argID+1)

	args = append(args, channelID, userID)

	tag, err := db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("channel not found")
	}

	return nil
}

func buildUpdateQuery(userID int, dto dto.UpdateUserRequest) (string, []any, error) {
	var setParts []string
	var args []any
	argID := 1

	if dto.Email != nil {
		setParts = append(setParts, fmt.Sprintf("email = $%d", argID))
		args = append(args, *dto.Email)
		argID++
	}

	if dto.Password != nil {
		setParts = append(setParts, fmt.Sprintf("password_hash = $%d", argID))
		args = append(args, *dto.Password)
		argID++
	}

	if len(setParts) == 0 {
		return "", nil, fmt.Errorf("no data")
	}

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(setParts, ", "), argID)

	args = append(args, userID)

	return query, args, nil
}

func GetUserIDByEmail(ctx context.Context, db *pgxpool.Pool, email string) (int, error) {
	var id int

	query := `SELECT id FROM users WHERE email = $1`

	err := db.QueryRow(ctx, query, email).Scan(&id)
	if err != nil {

		if err.Error() == "no rows in result set" {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}
