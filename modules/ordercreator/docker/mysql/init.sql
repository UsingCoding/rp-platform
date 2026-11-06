CREATE DATABASE IF NOT EXISTS `ordercreator` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'ordercreator'@'%' IDENTIFIED BY 'ordercreator';
GRANT ALL PRIVILEGES ON `ordercreator`.* TO 'ordercreator'@'%';
