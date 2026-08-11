-- 创建数据库
CREATE DATABASE IF NOT EXISTS chatroom DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE chatroom;

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `username` VARCHAR(50) NOT NULL COMMENT '用户名',
    `password` VARCHAR(255) NOT NULL COMMENT '密码(加密)',
    `nickname` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '昵称',
    `avatar` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '头像URL',
    `email` VARCHAR(100) DEFAULT NULL COMMENT '邮箱',
    `phone` VARCHAR(20) DEFAULT NULL COMMENT '手机号',
    `signature` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '个性签名',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=正常',
    `last_login` DATETIME DEFAULT NULL COMMENT '最后登录时间',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at` DATETIME DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`),
    UNIQUE KEY `uk_email` (`email`),
    KEY `idx_status` (`status`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- 好友关系表
CREATE TABLE IF NOT EXISTS `friends` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `friend_id` BIGINT UNSIGNED NOT NULL COMMENT '好友ID',
    `remark` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '好友备注',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_friend` (`user_id`, `friend_id`),
    KEY `idx_friend_id` (`friend_id`),
    CONSTRAINT `fk_friends_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
    CONSTRAINT `fk_friends_friend` FOREIGN KEY (`friend_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系表';

-- 群组表
CREATE TABLE IF NOT EXISTS `groups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(100) NOT NULL COMMENT '群名称',
    `owner_id` BIGINT UNSIGNED NOT NULL COMMENT '群主ID',
    `avatar` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '群头像',
    `description` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '群描述',
    `announcement` TEXT COMMENT '群公告',
    `max_members` INT UNSIGNED NOT NULL DEFAULT 500 COMMENT '最大成员数',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=已解散 1=正常',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at` DATETIME DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_owner_id` (`owner_id`),
    KEY `idx_deleted_at` (`deleted_at`),
    CONSTRAINT `fk_groups_owner` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组表';

-- 群成员表
CREATE TABLE IF NOT EXISTS `group_members` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `group_id` BIGINT UNSIGNED NOT NULL COMMENT '群组ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `nickname` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '群内昵称',
    `role` TINYINT NOT NULL DEFAULT 0 COMMENT '角色: 0=普通成员 1=管理员 2=群主',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=已退出 1=正常',
    `joined_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_group_user` (`group_id`, `user_id`),
    KEY `idx_user_id` (`user_id`),
    CONSTRAINT `fk_group_members_group` FOREIGN KEY (`group_id`) REFERENCES `groups` (`id`),
    CONSTRAINT `fk_group_members_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员表';

-- 消息表
CREATE TABLE IF NOT EXISTS `messages` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `msg_id` VARCHAR(36) NOT NULL COMMENT '消息UUID',
    `from_user_id` BIGINT UNSIGNED NOT NULL COMMENT '发送者ID',
    `to_id` BIGINT UNSIGNED NOT NULL COMMENT '接收者ID(用户ID或群组ID)',
    `to_type` ENUM('user', 'group') NOT NULL COMMENT '接收类型: user=私聊 group=群聊',
    `content_type` ENUM('text', 'image', 'file', 'system') NOT NULL DEFAULT 'text' COMMENT '内容类型',
    `content` TEXT NOT NULL COMMENT '消息内容',
    `extra` JSON DEFAULT NULL COMMENT '扩展信息',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_msg_id` (`msg_id`),
    KEY `idx_from_user` (`from_user_id`),
    KEY `idx_to_user` (`to_id`, `to_type`),
    KEY `idx_created_at` (`created_at`),
    CONSTRAINT `fk_messages_from_user` FOREIGN KEY (`from_user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息表';

-- 文件表
CREATE TABLE IF NOT EXISTS `files` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `filename` VARCHAR(255) NOT NULL COMMENT '原始文件名',
    `filepath` VARCHAR(500) NOT NULL COMMENT '存储路径',
    `filesize` BIGINT UNSIGNED NOT NULL COMMENT '文件大小(字节)',
    `filetype` VARCHAR(50) NOT NULL COMMENT '文件类型',
    `mimetype` VARCHAR(100) NOT NULL COMMENT 'MIME类型',
    `md5` VARCHAR(32) NOT NULL COMMENT '文件MD5',
    `uploader_id` BIGINT UNSIGNED NOT NULL COMMENT '上传者ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_uploader` (`uploader_id`),
    KEY `idx_md5` (`md5`),
    CONSTRAINT `fk_files_uploader` FOREIGN KEY (`uploader_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文件表';
