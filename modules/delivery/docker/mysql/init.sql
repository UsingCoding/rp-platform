CREATE DATABASE IF NOT EXISTS `delivery` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'delivery'@'%' IDENTIFIED BY 'delivery';
GRANT ALL PRIVILEGES ON `delivery`.* TO 'delivery'@'%';
