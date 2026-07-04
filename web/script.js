// Doodle Royale client. Handles the lobby (create/join a private room), then
// renders the game state pushed by the Go engine and publishes the local
// player's drawing, guesses, chat, and word choice.
window.addEventListener('DOMContentLoaded', () => {
    const canvas = document.getElementById('paintCanvas');
    if (!canvas) {
        console.error('Canvas element #paintCanvas not found');
        return;
    }
    const ctx = canvas.getContext('2d');
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';

    // The canvas backing store is a fixed logical resolution (see width/height
    // attributes in the HTML, mirrored by the server's LogicalWidth/Height).
    // CSS scales it to fit the window, so drawings stay aligned across clients
    // and survive window resizes without being cleared.
    const ERASER_COLOR = '#000000'; // matches the canvas background

    // --- Local state ---------------------------------------------------------
    const local = {
        painting: false,
        lastPos: null,
        color: '#39ff14',
        size: 5,
        eraser: false,
        canDraw: false, // true only while we are the drawer in the drawing phase
        name: 'Player',
    };

    let socket = null;

    // --- Elements ------------------------------------------------------------
    const el = {
        lobbyOverlay: document.getElementById('lobbyOverlay'),
        nameInput: document.getElementById('nameInput'),
        createBtn: document.getElementById('createBtn'),
        codeInput: document.getElementById('codeInput'),
        joinBtn: document.getElementById('joinBtn'),
        lobbyError: document.getElementById('lobbyError'),
        roomChip: document.getElementById('roomChip'),
        roundInfo: document.getElementById('roundInfo'),
        wordSlots: document.getElementById('wordSlots'),
        timer: document.getElementById('timer'),
        statusBanner: document.getElementById('statusBanner'),
        playerList: document.getElementById('playerList'),
        chatBox: document.getElementById('chatBox'),
        chatInput: document.getElementById('chatInput'),
        sendBtn: document.getElementById('sendBtn'),
        toolbar: document.getElementById('toolbar'),
        pencilBtn: document.getElementById('pencilBtn'),
        eraserBtn: document.getElementById('eraserBtn'),
        colorPicker: document.getElementById('colorPicker'),
        brushSize: document.getElementById('brushSize'),
        clearBtn: document.getElementById('clearBtn'),
        canvasOverlay: document.getElementById('canvasOverlay'),
        choiceOverlay: document.getElementById('choiceOverlay'),
        choiceButtons: document.getElementById('choiceButtons'),
    };

    function send(payload) {
        if (socket && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify(payload));
        }
    }

    // --- Lobby: create / join a room ----------------------------------------
    async function createRoom() {
        el.lobbyError.textContent = '';
        try {
            const res = await fetch('/api/rooms', { method: 'POST' });
            if (!res.ok) throw new Error(`server responded ${res.status}`);
            const { code } = await res.json();
            connect(code);
        } catch (err) {
            el.lobbyError.textContent = 'Could not create a room. Try again.';
            console.error(err);
        }
    }

    function joinRoom() {
        el.lobbyError.textContent = '';
        const code = el.codeInput.value.trim().toUpperCase();
        if (!code) {
            el.lobbyError.textContent = 'Enter a room code to join.';
            return;
        }
        connect(code);
    }

    function connect(code) {
        local.name = el.nameInput.value.trim() || 'Player';
        const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
        socket = new WebSocket(`${wsProtocol}://${window.location.host}/ws?room=${encodeURIComponent(code)}`);

        socket.addEventListener('open', () => {
            send({ type: 'join', name: local.name });
            el.roomChip.textContent = code;
            el.lobbyOverlay.classList.add('hidden');
            setBanner('Connected. Waiting for the game…');
            if (el.chatInput) el.chatInput.focus();
        });
        socket.addEventListener('close', () => {
            if (!el.lobbyOverlay.classList.contains('hidden')) return;
            setBanner('Disconnected. Refresh to reconnect.');
        });
        socket.addEventListener('message', (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch {
                return;
            }
            handleServerMessage(msg);
        });
    }

    el.createBtn.addEventListener('click', createRoom);
    el.joinBtn.addEventListener('click', joinRoom);
    el.codeInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') joinRoom(); });
    el.nameInput.focus();

    // --- Server messages -----------------------------------------------------
    function handleServerMessage(msg) {
        switch (msg.type) {
            case 'state':
                renderState(msg);
                break;
            case 'draw':
                drawSegment(msg);
                break;
            case 'clear':
                clearCanvas();
                break;
            case 'history':
                (msg.history || []).forEach(drawSegment);
                break;
            case 'chat':
                appendChat(msg);
                break;
            case 'timer':
                setTimer(msg.content);
                break;
            case 'choices':
                showChoices(msg.choices || []);
                break;
            case 'error':
                showLobbyError(msg.content || 'Something went wrong.');
                break;
        }
    }

    function showLobbyError(text) {
        if (socket) socket.close();
        el.lobbyOverlay.classList.remove('hidden');
        el.roomChip.textContent = '';
        el.lobbyError.textContent = text;
    }

    // --- State rendering -----------------------------------------------------
    function renderState(s) {
        local.canDraw = !!s.isDrawer && s.phase === 'drawing';
        if (s.phase !== 'choosing') hideChoiceOverlay();
        renderPlayers(s.players || [], s.youId);
        renderHeader(s);
        renderStatus(s);
        updateToolbarLock();
    }

    function renderHeader(s) {
        el.roundInfo.textContent = (s.round && s.maxRounds) ? `Round ${s.round}/${s.maxRounds}` : '';
        el.wordSlots.textContent = (s.word || '').split('').join(' ');
    }

    function renderStatus(s) {
        switch (s.phase) {
            case 'waiting':
                setBanner('Waiting for more players…');
                showCanvasOverlay('Waiting for players…\nShare the room code to invite friends.');
                break;
            case 'choosing':
                if (s.isDrawer) {
                    setBanner('Pick a word to draw!');
                    hideCanvasOverlay();
                } else {
                    setBanner(`${s.drawerName} is choosing a word…`);
                    showCanvasOverlay(`${s.drawerName} is choosing a word…`);
                }
                break;
            case 'drawing':
                setBanner(s.isDrawer ? `Your turn! Draw: ${s.word}` : `${s.drawerName} is drawing — start guessing!`);
                hideCanvasOverlay();
                break;
            case 'reveal':
                setBanner(`The word was "${s.word}"`);
                showCanvasOverlay(`The word was:\n${s.word}`);
                break;
            case 'gameover':
                setBanner('Game over!');
                showCanvasOverlay(gameOverText(s.players || []));
                break;
            default:
                setBanner('');
                hideCanvasOverlay();
        }
    }

    function gameOverText(players) {
        const board = players.map((p, i) => `${i + 1}. ${p.name} — ${p.score}`).join('\n');
        return `Game over!\n\n${board}`;
    }

    function renderPlayers(players, youId) {
        el.playerList.replaceChildren();
        for (const p of players) {
            const row = document.createElement('div');
            row.className = 'player-row';
            if (p.id === youId) row.classList.add('you');
            if (p.guessed) row.classList.add('guessed');

            const name = document.createElement('span');
            name.className = 'player-name';
            if (p.drawing) {
                const icon = document.createElement('i');
                icon.className = 'fas fa-pencil-alt';
                name.appendChild(icon);
            } else if (p.guessed) {
                const icon = document.createElement('i');
                icon.className = 'fas fa-check';
                name.appendChild(icon);
            }
            const label = document.createElement('span');
            label.textContent = p.name + (p.id === youId ? ' (you)' : '');
            name.appendChild(label);

            const score = document.createElement('span');
            score.className = 'player-score';
            score.textContent = p.score;

            row.append(name, score);
            el.playerList.appendChild(row);
        }
    }

    function setBanner(text) {
        el.statusBanner.textContent = text;
    }

    function setTimer(seconds) {
        el.timer.replaceChildren();
        const icon = document.createElement('i');
        icon.className = 'fas fa-clock';
        el.timer.append(icon, ` ${seconds}s`);
    }

    function showCanvasOverlay(text) {
        el.canvasOverlay.textContent = text;
        el.canvasOverlay.classList.remove('hidden');
    }

    function hideCanvasOverlay() {
        el.canvasOverlay.classList.add('hidden');
    }

    // --- Word choice (drawer) ------------------------------------------------
    function showChoices(words) {
        el.choiceButtons.replaceChildren();
        for (const word of words) {
            const btn = document.createElement('button');
            btn.textContent = word;
            btn.addEventListener('click', () => {
                send({ type: 'pick', content: word });
                hideChoiceOverlay();
            });
            el.choiceButtons.appendChild(btn);
        }
        el.choiceOverlay.classList.remove('hidden');
    }

    function hideChoiceOverlay() {
        el.choiceOverlay.classList.add('hidden');
    }

    // --- Drawing -------------------------------------------------------------
    function drawSegment(seg) {
        ctx.beginPath();
        ctx.strokeStyle = seg.color;
        ctx.lineWidth = seg.size;
        ctx.moveTo(seg.prevX, seg.prevY);
        ctx.lineTo(seg.x, seg.y);
        ctx.stroke();
    }

    function clearCanvas() {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    }

    function getPos(e) {
        const rect = canvas.getBoundingClientRect();
        return {
            x: ((e.clientX - rect.left) / rect.width) * canvas.width,
            y: ((e.clientY - rect.top) / rect.height) * canvas.height,
        };
    }

    function startStroke(e) {
        if (!local.canDraw) return;
        local.painting = true;
        local.lastPos = getPos(e);
    }

    function moveStroke(e) {
        if (!local.painting || !local.lastPos || !local.canDraw) return;
        const pos = getPos(e);
        const segment = {
            type: 'draw',
            prevX: local.lastPos.x,
            prevY: local.lastPos.y,
            x: pos.x,
            y: pos.y,
            color: local.eraser ? ERASER_COLOR : local.color,
            size: local.eraser ? Math.max(local.size * 2, 20) : local.size,
        };
        drawSegment(segment); // instant local feedback
        send(segment);
        local.lastPos = pos;
    }

    function endStroke() {
        local.painting = false;
        local.lastPos = null;
    }

    canvas.addEventListener('mousedown', startStroke);
    canvas.addEventListener('mousemove', moveStroke);
    window.addEventListener('mouseup', endStroke);
    canvas.addEventListener('touchstart', (e) => { e.preventDefault(); startStroke(e.touches[0]); });
    canvas.addEventListener('touchmove', (e) => { e.preventDefault(); moveStroke(e.touches[0]); });
    canvas.addEventListener('touchend', endStroke);

    // --- Toolbar -------------------------------------------------------------
    function setTool(useEraser) {
        local.eraser = useEraser;
        el.pencilBtn.classList.toggle('active', !useEraser);
        el.eraserBtn.classList.toggle('active', useEraser);
    }

    function updateToolbarLock() {
        el.toolbar.classList.toggle('disabled', !local.canDraw);
        canvas.classList.toggle('locked', !local.canDraw);
    }

    el.pencilBtn.addEventListener('click', () => setTool(false));
    el.eraserBtn.addEventListener('click', () => setTool(true));
    el.colorPicker.addEventListener('input', (e) => { local.color = e.target.value; });
    el.brushSize.addEventListener('input', (e) => { local.size = Number(e.target.value); });
    el.clearBtn.addEventListener('click', () => {
        if (!local.canDraw) return;
        send({ type: 'clear' });
        clearCanvas();
    });

    // --- Chat ----------------------------------------------------------------
    // Built with textContent so chat/guess text can never be interpreted as
    // HTML (prevents cross-site scripting).
    function appendChat(msg) {
        const line = document.createElement('div');
        line.className = 'chat-entry';

        if (msg.kind === 'system') {
            line.classList.add('system');
            line.textContent = msg.content;
        } else if (msg.kind === 'correct') {
            line.classList.add('correct');
            line.textContent = msg.content;
        } else {
            const author = document.createElement('span');
            author.className = 'author';
            author.textContent = `${msg.sender}: `;
            const body = document.createElement('span');
            body.textContent = msg.content;
            line.append(author, body);
        }

        el.chatBox.appendChild(line);
        el.chatBox.scrollTop = el.chatBox.scrollHeight;
    }

    function sendChat() {
        const text = el.chatInput.value.trim();
        if (!text) return;
        send({ type: 'chat', content: text });
        el.chatInput.value = '';
    }

    el.sendBtn.addEventListener('click', sendChat);
    el.chatInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') sendChat();
    });

    // --- Canvas sizing -------------------------------------------------------
    function resizeCanvas() {
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
    }
    window.addEventListener('resize', resizeCanvas);

    updateToolbarLock();
});
