# Отправляем серию запросов (замените :8080 на ваш порт)
for i in {1..110}; do
  echo "Request $i:"
  curl -I http://localhost:8080/ping
  sleep 0.1
done