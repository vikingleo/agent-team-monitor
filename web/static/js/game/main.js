import { GameConfig } from './config.js';

class Game {
    constructor() {
        this.game = null;
    }

    init() {
        this.game = new Phaser.Game(GameConfig);
        console.log('Phaser game initialized');
    }

    destroy() {
        if (this.game) {
            this.game.destroy(true);
        }
    }
}

// 全局实例
window.AgentMonitorGame = new Game();

// 页面加载后初始化
document.addEventListener('DOMContentLoaded', () => {
    window.AgentMonitorGame.init();
});
