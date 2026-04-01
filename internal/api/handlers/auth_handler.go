package handlers

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ghduuep/pingly/internal/database"
	"github.com/ghduuep/pingly/internal/dto"
	"github.com/ghduuep/pingly/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// @Summary Register a new user
// @Description Create a new user account using username, email, and password.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Register Credentials"
// @Success 201 {object} nil
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /register [post]
func (h *Handler) Register(c echo.Context) error {
	var req dto.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid data."})
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to proccess data."})
	}

	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}

	if err := database.CreateUser(c.Request().Context(), h.DB, &user); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User already exists."})
	}

	defaultChannel := models.NotificationChannel{
		UserID:  user.ID,
		Type:    models.TypeEmail,
		Target:  user.Email,
		Enabled: true,
	}

	if err := database.CreateChannel(c.Request().Context(), h.DB, &defaultChannel); err != nil {
		log.Printf("[ERROR] Failed to create default channel for user %d: %v", user.ID, err)
	}

	return c.NoContent(http.StatusCreated)
}

// @Summary Login user
// @Description Authenticate a user and return a JWT token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login Credentials"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /login [post]
func (h *Handler) Login(c echo.Context) error {
	var req dto.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid data."})
	}

	user, err := database.GetUserByUsername(c.Request().Context(), h.DB, req.Username)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials."})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials."})
	}

	claims := jwt.MapClaims{
		"sub":  user.ID,
		"name": user.Username,
		"exp":  time.Now().Add(time.Hour * 72).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")

	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate the token."})
	}

	return c.JSON(http.StatusOK, map[string]string{"token": t})
}

func extractTokenString(c echo.Context) string {
	authHeader := c.Request().Header.Get("Authorization")
	if len(authHeader) > 7 && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

// @Summary Logout user
// @Description Invalida o token JWT atual adicionando-o à lista negra no Redis.
// @Tags auth
// @Security BearerAuth
// @Router /logout [post]
func (h *Handler) Logout(c echo.Context) error {
	tokenString := extractTokenString(c)
	if tokenString == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token is missing."})
	}

	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid token format."})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to parse claims."})
	}

	var ttl time.Duration
	if exp, ok := claims["exp"].(float64); ok {
		expirationTime := time.Unix(int64(exp), 0)
		ttl = time.Until(expirationTime)
	}

	if ttl <= 0 {
		ttl = 1 * time.Second
	}

	if err := database.AddToBlackList(c.Request().Context(), h.RDB, tokenString, ttl); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to logout."})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Logout successful."})
}
