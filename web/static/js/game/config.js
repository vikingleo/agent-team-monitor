import { OfficeScene } from './scenes/OfficeScene.js';

export const GameConfig = {
    type: Phaser.AUTO,
    parent: 'game-container',
    width: window.innerWidth - 490,  // 减去侧栏宽度 (400px) + tab 宽度 (90px)
    height: window.innerHeight,
    backgroundColor: '#f5f5f5',
    scene: [OfficeScene],
    physics: {
        default: 'arcade',
        arcade: {
            debug: false
        }
    },
    scale: {
        mode: Phaser.Scale.RESIZE,
        autoCenter: Phaser.Scale.CENTER_BOTH,
        resolution: window.devicePixelRatio || 1  // 修复模糊问题
    },
    render: {
        pixelArt: false,
        antialias: true,
        roundPixels: false
    }
};

export const Constants = {
    POLL_INTERVAL: 1000,
    AGENT_SPEED: 100,
    AGENT_IDLE_SPEED: 50,
    AGENT_SIZE: 40,
    OFFICE_MIN_WIDTH: 200,
    OFFICE_MIN_HEIGHT: 150,
    ROOM_PADDING: 20,
    CORRIDOR_WIDTH: 150
};
