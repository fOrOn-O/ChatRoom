-- 一次性迁移：移除已读回执和消息撤回的数据结构。
--
-- 执行前请备份数据库。删除 status 列之前必须先清理非正常消息，
-- 否则原先仅靠 status 隐藏的消息内容会重新出现在历史记录中。

USE chatroom;

DELETE FROM `messages`
WHERE `status` <> 1;

ALTER TABLE `messages`
    DROP COLUMN `status`;

DROP TABLE IF EXISTS `read_receipts`;
