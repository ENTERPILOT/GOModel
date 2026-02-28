(function(global) {
    function dashboardDatePickerModule() {
        return {
            toggleDatePicker() {
                this.datePickerOpen = !this.datePickerOpen;
                if (this.datePickerOpen) {
                    this.calendarMonth = new Date();
                    this.selectingDate = 'start';
                }
            },

            closeDatePicker() {
                this.datePickerOpen = false;
                this.cursorHint.show = false;
            },

            onCalendarMouseMove(e) {
                this.cursorHint = { show: true, x: e.clientX, y: e.clientY };
            },

            onCalendarMouseLeave() {
                this.cursorHint.show = false;
            },

            selectPreset(days) {
                this.selectedPreset = days;
                this.customStartDate = null;
                this.customEndDate = null;
                this.selectingDate = 'start';
                this.days = days;
                this.fetchUsage();
                this.closeDatePicker();
            },

            selectionHint() {
                return this.selectingDate === 'end' ? 'Select end date' : 'Select start date';
            },

            dateRangeLabel() {
                if (this.selectedPreset) return 'Last ' + this.selectedPreset + ' days';
                if (this.customStartDate && this.customEndDate) {
                    return this.formatDateShort(this.customStartDate) + ' \u2013 ' + this.formatDateShort(this.customEndDate);
                }
                if (this.customStartDate) {
                    return this.formatDateShort(this.customStartDate) + ' \u2013 ...';
                }
                return 'Last 30 days';
            },

            formatDateShort(date) {
                const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
                return months[date.getMonth()] + ' ' + date.getDate() + ', ' + date.getFullYear();
            },

            calendarTitle(offset) {
                const d = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() + offset, 1);
                const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
                return months[d.getMonth()] + ' ' + d.getFullYear();
            },

            calendarDays(offset) {
                const year = this.calendarMonth.getFullYear();
                const month = this.calendarMonth.getMonth() + offset;
                const first = new Date(year, month, 1);
                const last = new Date(year, month + 1, 0);
                let startDay = (first.getDay() + 6) % 7;
                const days = [];

                const prevLast = new Date(year, month, 0);
                for (let i = startDay - 1; i >= 0; i--) {
                    const d = prevLast.getDate() - i;
                    days.push({ day: d, date: new Date(year, month - 1, d), current: false, key: 'p' + d });
                }

                for (let d = 1; d <= last.getDate(); d++) {
                    days.push({ day: d, date: new Date(year, month, d), current: true, key: 'c' + d });
                }

                const remaining = 42 - days.length;
                for (let d = 1; d <= remaining; d++) {
                    days.push({ day: d, date: new Date(year, month + 1, d), current: false, key: 'n' + d });
                }

                return days;
            },

            prevMonth() {
                this.calendarMonth = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() - 1, 1);
            },

            nextMonth() {
                const next = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() + 1, 1);
                const today = new Date();
                if (next.getFullYear() < today.getFullYear() ||
                    (next.getFullYear() === today.getFullYear() && next.getMonth() <= today.getMonth())) {
                    this.calendarMonth = next;
                }
            },

            isCurrentMonth() {
                const today = new Date();
                return this.calendarMonth.getFullYear() === today.getFullYear() &&
                    this.calendarMonth.getMonth() === today.getMonth();
            },

            selectCalendarDay(day) {
                if (!day.current || this.isFutureDay(day)) return;
                const clicked = new Date(day.date);
                clicked.setHours(0, 0, 0, 0);
                this.selectedPreset = null;

                if (this.selectingDate === 'start') {
                    this.customStartDate = clicked;
                    if (this.customEndDate && this.customEndDate < clicked) {
                        this.customEndDate = clicked;
                    }
                    if (!this.customEndDate) {
                        const today = new Date();
                        today.setHours(0, 0, 0, 0);
                        this.customEndDate = today;
                    }
                    this.selectingDate = 'end';
                    this.fetchUsage();
                } else {
                    if (clicked < this.customStartDate) {
                        this.customEndDate = this.customStartDate;
                        this.customStartDate = clicked;
                    } else {
                        this.customEndDate = clicked;
                    }
                    this.selectingDate = 'start';
                    this.fetchUsage();
                    this.closeDatePicker();
                }
            },

            isToday(day) {
                if (!day.current) return false;
                const today = new Date();
                return day.date.getFullYear() === today.getFullYear() &&
                    day.date.getMonth() === today.getMonth() &&
                    day.date.getDate() === today.getDate();
            },

            isFutureDay(day) {
                const today = new Date();
                today.setHours(23, 59, 59, 999);
                return day.date > today;
            },

            isRangeStart(day) {
                if (!day.current) return false;
                const start = this._rangeStart();
                if (!start) return false;
                return day.date.getFullYear() === start.getFullYear() &&
                    day.date.getMonth() === start.getMonth() &&
                    day.date.getDate() === start.getDate();
            },

            isRangeEnd(day) {
                if (!day.current) return false;
                const end = this._rangeEnd();
                if (!end) return false;
                return day.date.getFullYear() === end.getFullYear() &&
                    day.date.getMonth() === end.getMonth() &&
                    day.date.getDate() === end.getDate();
            },

            isInRange(day) {
                if (!day.current) return false;
                const start = this._rangeStart();
                const end = this._rangeEnd();
                if (!start || !end) return false;
                const dayDate = new Date(day.date);
                dayDate.setHours(0, 0, 0, 0);
                return dayDate >= start && dayDate <= end;
            },

            _rangeStart() {
                if (this.customStartDate) return this.customStartDate;
                if (this.selectedPreset) {
                    const s = new Date();
                    s.setHours(0, 0, 0, 0);
                    s.setDate(s.getDate() - (parseInt(this.selectedPreset, 10) - 1));
                    return s;
                }
                return null;
            },

            _rangeEnd() {
                if (this.customEndDate) return this.customEndDate;
                if (this.customStartDate || this.selectedPreset) {
                    const t = new Date();
                    t.setHours(0, 0, 0, 0);
                    return t;
                }
                return null;
            },

            setInterval(val) {
                this.interval = val;
                this.fetchUsage();
            },

            chartTitle() {
                const titles = { daily: 'Daily', weekly: 'Weekly', monthly: 'Monthly', yearly: 'Yearly' };
                return (titles[this.interval] || 'Daily') + ' Token Usage';
            }
        };
    }

    global.dashboardDatePickerModule = dashboardDatePickerModule;
})(window);
