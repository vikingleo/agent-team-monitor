export class DrawFunctions {
    static drawAgent(graphics, x, y, state) {
        const color = state === 'busy' ? 0x4CAF50 : 0x9E9E9E;

        // 身体（圆形）
        graphics.fillStyle(color, 1);
        graphics.fillCircle(x, y, 20);

        // 眼睛
        graphics.fillStyle(0x000000, 1);
        graphics.fillCircle(x - 8, y - 5, 3);
        graphics.fillCircle(x + 8, y - 5, 3);

        // 微笑
        graphics.lineStyle(2, 0x000000);
        graphics.beginPath();
        graphics.arc(x, y + 5, 10, 0, Math.PI, false);
        graphics.strokePath();
    }

    static drawAgentName(graphics, x, y, name) {
        const text = graphics.scene.add.text(x, y + 30, name, {
            fontSize: '12px',
            color: '#333',
            align: 'center'
        });
        text.setOrigin(0.5, 0);
        return text;
    }

    static drawOffice(graphics, x, y, width, height, teamName) {
        // 墙壁
        graphics.fillStyle(0xE8E8E8, 1);
        graphics.fillRect(x, y, width, height);

        // 边框
        graphics.lineStyle(3, 0x666666);
        graphics.strokeRect(x, y, width, height);

        // 门（底部中央）
        graphics.fillStyle(0x8B4513, 1);
        graphics.fillRect(x + width/2 - 20, y + height - 5, 40, 5);

        // 窗户（左上角）
        graphics.lineStyle(2, 0x87CEEB);
        graphics.strokeRect(x + 10, y + 10, 40, 30);

        // 团队名称
        const text = graphics.scene.add.text(x + width/2, y + 20, teamName, {
            fontSize: '14px',
            fontStyle: 'bold',
            color: '#333',
            align: 'center'
        });
        text.setOrigin(0.5, 0);
        return text;
    }

    static drawFacility(graphics, x, y, width, height, type) {
        // 背景
        graphics.fillStyle(0xFFFFFF, 1);
        graphics.fillRect(x, y, width, height);

        // 边框
        graphics.lineStyle(2, 0x999999);
        graphics.strokeRect(x, y, width, height);

        // 根据类型绘制图标和文字
        let emoji, label;
        switch(type) {
            case 'restroom':
                emoji = '🚻';
                label = '洗手间';
                break;
            case 'cafe':
                emoji = '☕';
                label = '茶水间';
                break;
            case 'gym':
                emoji = '💪';
                label = '健身区';
                break;
            case 'boss':
                emoji = '🏢';
                label = '老板办公室';
                break;
            default:
                emoji = '📦';
                label = type;
        }

        const emojiText = graphics.scene.add.text(x + width/2, y + height/2 - 15, emoji, {
            fontSize: '32px',
            align: 'center'
        });
        emojiText.setOrigin(0.5, 0.5);

        const labelText = graphics.scene.add.text(x + width/2, y + height/2 + 20, label, {
            fontSize: '12px',
            color: '#666',
            align: 'center'
        });
        labelText.setOrigin(0.5, 0.5);

        return { emojiText, labelText };
    }

    static drawBubble(graphics, x, y, emoji) {
        // 白色圆角矩形
        graphics.fillStyle(0xFFFFFF, 0.95);
        graphics.fillRoundedRect(x - 20, y - 40, 40, 30, 8);

        // 边框
        graphics.lineStyle(1, 0xCCCCCC);
        graphics.strokeRoundedRect(x - 20, y - 40, 40, 30, 8);

        // Emoji
        const text = graphics.scene.add.text(x, y - 25, emoji, {
            fontSize: '20px',
            align: 'center'
        });
        text.setOrigin(0.5, 0.5);
        return text;
    }
}
