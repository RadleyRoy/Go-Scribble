window.addEventListener('DOMContentLoaded', () => {
    const canvas = document.getElementById('paintCanvas');
    if (!canvas) {
        console.error("Canvas not found");
        return;
    }

    const ctx = canvas.getContext('2d');
    const socket = new WebSocket(`ws://${window.location.host}/ws`);

    let painting = false;
    let lastPos = null;
    window.activeColor = '#39ff14';
    let isEraser = false;

    function initCanvas() {
        const rect = canvas.parentNode.getBoundingClientRect();
        canvas.width = rect.width;
        canvas.height = rect.height;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
    }

    function getMousePos(e) {
        const rect = canvas.getBoundingClientRect();
        return { x: e.clientX - rect.left, y: e.clientY - rect.top };
    }

    canvas.onmousedown = (e) => { painting = true; lastPos = getMousePos(e); };
    window.onmouseup = () => { painting = false; lastPos = null; };

    canvas.onmousemove = (e) => {
        if (!painting) return;

        const pos = getMousePos(e);

        if (lastPos) {
            const color = isEraser ? '#000000' : window.activeColor;
            const size = isEraser ? 25 : 5;

            draw(lastPos.x, lastPos.y, pos.x, pos.y, color, size);

            if (socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({
                    type: 'draw',
                    x: pos.x,
                    y: pos.y,
                    prevX: lastPos.x,
                    prevY: lastPos.y,
                    color,
                    size
                }));
            }
        }

        lastPos = pos;
    };

    function draw(x1, y1, x2, y2, color, size) {
        ctx.beginPath();
        ctx.strokeStyle = color;
        ctx.lineWidth = size;
        ctx.moveTo(x1, y1);
        ctx.lineTo(x2, y2);
        ctx.stroke();
    }

    socket.onmessage = (e) => {
        const msg = JSON.parse(e.data);

        if (msg.type === 'draw')
            draw(msg.prevX, msg.prevY, msg.x, msg.y, msg.color, msg.size);

        if (msg.type === 'clear')
            ctx.clearRect(0, 0, canvas.width, canvas.height);
    };

    window.setTool = (tool) => {
        isEraser = (tool === 'eraser');

        document.getElementById('pencilBtn').classList.toggle('active', !isEraser);
        document.getElementById('eraserBtn').classList.toggle('active', isEraser);
    };

    window.clearCanvas = () => {
        socket.send(JSON.stringify({ type: 'clear' }));
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    };

    window.addEventListener('resize', initCanvas);

    initCanvas();
});