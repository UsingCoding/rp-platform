CREATE DATABASE IF NOT EXISTS `payment` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'payment'@'%' IDENTIFIED BY 'payment';
GRANT ALL PRIVILEGES ON `payment`.* TO 'payment'@'%';
