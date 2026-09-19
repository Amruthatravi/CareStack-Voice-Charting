package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	TenantID  string `json:"tenant_id"`
	PatientID string `json:"patient_id"`
	VisitID   string `json:"visit_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func ValidateToken(tokenStr, secret string) (*Claims, error) {
	if tokenStr == "" {
		return nil, errors.New("empty token")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

func ExtractFromRequest(r *http.Request, secret string) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	var tokenStr string

	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		tokenStr = r.URL.Query().Get("token")
	}

	if tokenStr == "" {
		return nil, errors.New("token not found in request")
	}

	return ValidateToken(tokenStr, secret)
}
